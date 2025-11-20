// Copyright 2025 Chainguard, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"chainguard.dev/apko/pkg/build/types"
	"github.com/chainguard-dev/clog"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/sign"
	"github.com/sigstore/cosign/v2/pkg/cosign/attestation"
	cremote "github.com/sigstore/cosign/v2/pkg/cosign/remote"
	"github.com/sigstore/cosign/v2/pkg/oci/mutate"
	ociremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	"github.com/sigstore/cosign/v2/pkg/oci/static"
	ctypes "github.com/sigstore/cosign/v2/pkg/types"
	"github.com/sigstore/sigstore/pkg/signature/dsse"
	signatureoptions "github.com/sigstore/sigstore/pkg/signature/options"
)

// AttestationOptions contains options for signing and attestation
type AttestationOptions struct {
	// Enable signing and attestation
	Enabled bool

	// Path to private key for signing (empty = keyless/OIDC)
	KeyRef string

	// Skip logging to Rekor transparency log
	SkipTransparencyLog bool

	// Artifacts to generate attestations for
	Artifacts *Artifacts
}

// SignAndAttest signs the published image and attaches attestations
func (p *Publisher) SignAndAttest(ctx context.Context, imageRef string, opts *AttestationOptions) error {
	if opts == nil || !opts.Enabled {
		return nil
	}

	log := clog.FromContext(ctx)
	log.Infof("Signing and attesting image: %s", imageRef)

	// Parse image reference
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return fmt.Errorf("parsing image reference: %w", err)
	}

	// Sign the image first
	if err := p.signImage(ctx, ref, opts); err != nil {
		return fmt.Errorf("signing image: %w", err)
	}

	// Generate predicates for attestations
	if opts.Artifacts != nil {
		predicates, err := generatePredicates(opts.Artifacts)
		if err != nil {
			return fmt.Errorf("generating predicates: %w", err)
		}

		// Attach each predicate as an attestation
		for i, predicate := range predicates {
			log.Infof("Attaching attestation %d/%d: %s", i+1, len(predicates), predicate.Type)
			if err := p.attachAttestation(ctx, ref, predicate, opts); err != nil {
				return fmt.Errorf("attaching attestation %s: %w", predicate.Type, err)
			}
		}
	}

	log.Infof("Successfully signed and attested image: %s", imageRef)
	return nil
}

// signImage signs the image using cosign library
func (p *Publisher) signImage(ctx context.Context, ref name.Reference, opts *AttestationOptions) error {
	log := clog.FromContext(ctx)
	log.Infof("Signing image: %s", ref.String())

	// Configure signing options
	signOpts := options.SignOptions{
		Upload:           true,
		TlogUpload:       !opts.SkipTransparencyLog,
		SkipConfirmation: true,
		Recursive:        false,
	}

	// Configure key options
	keyOpts := options.KeyOpts{
		KeyRef:           opts.KeyRef,
		FulcioURL:        options.DefaultFulcioURL,
		OIDCIssuer:       options.DefaultOIDCIssuerURL,
		OIDCClientID:     "sigstore",
		RekorURL:         options.DefaultRekorURL,
		SkipConfirmation: true,
	}

	// Prepare signing command
	ro := options.RootOptions{
		Timeout: options.DefaultTimeout,
	}

	// Execute signing using cosign sign command
	if err := sign.SignCmd(&ro, keyOpts, signOpts, []string{ref.String()}); err != nil {
		return fmt.Errorf("cosign sign failed: %w", err)
	}

	log.Infof("Image signed successfully")
	return nil
}

// resolveDigestAndHash resolves a reference to a digest and returns the digest reference and hash
func (p *Publisher) resolveDigestAndHash(ref name.Reference, ociremoteOpts []ociremote.Option) (name.Digest, v1.Hash, error) {
	digest, err := ociremote.ResolveDigest(ref, ociremoteOpts...)
	if err != nil {
		return name.Digest{}, v1.Hash{}, fmt.Errorf("resolving digest: %w", err)
	}
	h, _ := v1.NewHash(digest.Identifier())
	return digest, h, nil
}

// generateSignedAttestation generates and signs an attestation payload using DSSE
func (p *Publisher) generateSignedAttestation(ctx context.Context, predicate Predicate, digest name.Digest, h v1.Hash, opts *AttestationOptions) ([]byte, *sign.SignerVerifier, mutate.DupeDetector, error) {
	// Marshal predicate data
	predicateJSON, err := json.Marshal(predicate.Data)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshaling predicate data: %w", err)
	}

	// Generate attestation statement
	stmt, err := attestation.GenerateStatement(attestation.GenerateOpts{
		Predicate: bytes.NewReader(predicateJSON),
		Type:      predicate.Type,
		Digest:    h.Hex,
		Repo:      digest.Repository.String(),
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generating statement: %w", err)
	}

	payload, err := json.Marshal(stmt)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshaling statement: %w", err)
	}

	// Get signer
	keyOpts := options.KeyOpts{
		KeyRef:           opts.KeyRef,
		FulcioURL:        options.DefaultFulcioURL,
		OIDCIssuer:       options.DefaultOIDCIssuerURL,
		OIDCClientID:     "sigstore",
		RekorURL:         options.DefaultRekorURL,
		SkipConfirmation: true,
	}
	sv, genKey, err := sign.SignerFromKeyOpts(ctx, "", "", keyOpts)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("creating signer: %w", err)
	}
	if genKey {
		sv, err = sign.KeylessSigner(ctx, keyOpts, sv)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("getting keyless signer: %w", err)
		}
	}

	// Wrap signer with DSSE
	wrapped := dsse.WrapSigner(sv, ctypes.IntotoPayloadType)
	dd := cremote.NewDupeDetector(sv)

	// Sign the payload
	signedPayload, err := wrapped.SignMessage(bytes.NewReader(payload), signatureoptions.WithContext(ctx))
	if err != nil {
		return nil, sv, nil, fmt.Errorf("signing payload: %w", err)
	}

	return signedPayload, sv, dd, nil
}

// attachAndWriteAttestation creates an attestation signature, attaches it to the entity, and writes it
func (p *Publisher) attachAndWriteAttestation(signedPayload []byte, predicate Predicate, digest name.Digest, sv *sign.SignerVerifier, dd mutate.DupeDetector, ociremoteOpts []ociremote.Option) error {
	// Create attestation signature
	staticOpts := []static.Option{
		static.WithLayerMediaType(ctypes.DssePayloadType),
		static.WithAnnotations(map[string]string{
			"predicateType": predicate.Type,
		}),
	}
	if sv.Cert != nil {
		staticOpts = append(staticOpts, static.WithCertChain(sv.Cert, sv.Chain))
	}

	sig, err := static.NewAttestation(signedPayload, staticOpts...)
	if err != nil {
		return fmt.Errorf("creating attestation: %w", err)
	}

	// Get signed entity
	se := ociremote.SignedUnknown(digest, ociremoteOpts...)

	// Attach attestation
	mutateOpts := []mutate.SignOption{
		mutate.WithDupeDetector(dd),
	}

	// Replace existing attestations of the same type
	ro := cremote.NewReplaceOp(predicate.Type)
	mutateOpts = append(mutateOpts, mutate.WithReplaceOp(ro))

	newSE, err := mutate.AttachAttestationToEntity(se, sig, mutateOpts...)
	if err != nil {
		return fmt.Errorf("attaching attestation to entity: %w", err)
	}

	// Write attestations
	if err := ociremote.WriteAttestations(digest.Repository, newSE, ociremoteOpts...); err != nil {
		return fmt.Errorf("writing attestations: %w", err)
	}

	return nil
}

// attachAttestation attaches an attestation predicate to the image using cosign library
func (p *Publisher) attachAttestation(ctx context.Context, ref name.Reference, predicate Predicate, opts *AttestationOptions) error {
	log := clog.FromContext(ctx)
	log.Infof("Attaching attestation with predicate type: %s", predicate.Type)

	ociremoteOpts := append([]ociremote.Option{}, ociremote.WithRemoteOptions(p.remoteOpts...))

	// Resolve digest
	digest, h, err := p.resolveDigestAndHash(ref, ociremoteOpts)
	if err != nil {
		return err
	}

	// Generate and sign attestation
	signedPayload, sv, dd, err := p.generateSignedAttestation(ctx, predicate, digest, h, opts)
	if err != nil {
		return err
	}
	defer sv.Close()

	// Attach and write attestation
	if err := p.attachAndWriteAttestation(signedPayload, predicate, digest, sv, dd, ociremoteOpts); err != nil {
		return err
	}

	log.Infof("Attestation attached successfully")
	return nil
}

// SignAndAttestIndex signs and attests an image index by processing each architecture
func (p *Publisher) SignAndAttestIndex(ctx context.Context, indexRef string, artifactsByArch map[types.Architecture]*Artifacts, keyRef string, skipTlog bool) error {
	log := clog.FromContext(ctx)

	// Parse the index reference
	ref, err := name.ParseReference(indexRef)
	if err != nil {
		return fmt.Errorf("parsing index reference: %w", err)
	}

	// Get the index manifest to find sub-indexes
	desc, err := remote.Get(ref, p.remoteOpts...)
	if err != nil {
		return fmt.Errorf("fetching index: %w", err)
	}

	if !desc.MediaType.IsIndex() {
		return fmt.Errorf("reference is not an index: %s", desc.MediaType)
	}

	idx, err := desc.ImageIndex()
	if err != nil {
		return fmt.Errorf("getting image index: %w", err)
	}

	manifest, err := idx.IndexManifest()
	if err != nil {
		return fmt.Errorf("getting index manifest: %w", err)
	}

	// Sign and attest each architecture's sub-index
	for _, desc := range manifest.Manifests {
		if desc.Platform == nil {
			log.Warnf("Skipping manifest without platform information")
			continue
		}

		// Find matching architecture in artifacts
		arch := types.ParseArchitecture(desc.Platform.Architecture)
		artifacts, ok := artifactsByArch[arch]
		if !ok {
			log.Warnf("No artifacts found for architecture %s, skipping attestation", arch.ToAPK())
			continue
		}

		// Build reference to this sub-index
		subRef := ref.Context().Digest(desc.Digest.String())

		log.Infof("Signing and attesting sub-index for %s: %s", arch.ToAPK(), subRef.String())

		// Sign and attest this sub-index
		attestOpts := &AttestationOptions{
			Enabled:             true,
			KeyRef:              keyRef,
			SkipTransparencyLog: skipTlog,
			Artifacts:           artifacts,
		}

		if err := p.SignAndAttest(ctx, subRef.String(), attestOpts); err != nil {
			return fmt.Errorf("signing/attesting %s sub-index: %w", arch.ToAPK(), err)
		}
	}

	// Also sign the top-level index itself
	log.Infof("Signing top-level index: %s", indexRef)
	topLevelOpts := &AttestationOptions{
		Enabled:             true,
		KeyRef:              keyRef,
		SkipTransparencyLog: skipTlog,
		Artifacts:           nil, // No per-arch artifacts for top-level
	}

	// For top-level, we only sign (no attestations since it's multi-arch)
	if err := p.signImage(ctx, ref, topLevelOpts); err != nil {
		return fmt.Errorf("signing top-level index: %w", err)
	}

	log.Infof("Successfully signed and attested all images")
	return nil
}
