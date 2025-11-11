package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
)

type uploadDiskImageOpts struct {
	imagePath             string
	diskName              string
	sizeGB                int
	resourceGroup         string
	region                string
	arch                  armcompute.Architecture
	tags                  map[string]*string
	acceleratedNetworking bool
}

func uploadDiskImage(ctx context.Context, clients *AzureClients, opts *uploadDiskImageOpts, verbose bool) (string, error) {
	vhdPath, cleanup, err := prepareVHDImage(opts.imagePath, opts.sizeGB, verbose)
	if err != nil {
		return "", fmt.Errorf("failed to prepare VHD image: %w", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	vhdSize, err := getFileSize(vhdPath)
	if err != nil {
		return "", fmt.Errorf("failed to get VHD size: %w", err)
	}

	if verbose {
		log.Printf("VHD file size: %d bytes (%.2f MB)", vhdSize, float64(vhdSize)/(1024*1024))
	}

	disk := armcompute.Disk{
		Location: to.Ptr(opts.region),
		Properties: &armcompute.DiskProperties{
			CreationData: &armcompute.CreationData{
				CreateOption:    to.Ptr(armcompute.DiskCreateOptionUpload),
				UploadSizeBytes: to.Ptr(int64(vhdSize)),
			},
			OSType:           to.Ptr(armcompute.OperatingSystemTypesLinux),
			HyperVGeneration: to.Ptr(armcompute.HyperVGenerationV2),
			SupportedCapabilities: &armcompute.SupportedCapabilities{
				Architecture:       &opts.arch,
				AcceleratedNetwork: &opts.acceleratedNetworking,
			},
		},
		SKU: &armcompute.DiskSKU{
			Name: to.Ptr(armcompute.DiskStorageAccountTypesStandardLRS),
		},
		Tags: opts.tags,
	}

	log.Printf("Creating disk: %s", opts.diskName)

	poller, err := clients.Disks.BeginCreateOrUpdate(ctx, opts.resourceGroup, opts.diskName, disk, nil)
	if err != nil {
		return "", fmt.Errorf("failed to start disk creation: %w", err)
	}

	diskResult, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create disk: %w", err)
	}

	diskID := *diskResult.ID

	if verbose {
		log.Printf("Disk created: %s", diskID)
	}

	accessDuration := int32(86400) // 24 hours
	accessReq := armcompute.GrantAccessData{
		Access:            to.Ptr(armcompute.AccessLevelWrite),
		DurationInSeconds: &accessDuration,
	}

	log.Printf("Granting write access to disk")

	accessPoller, err := clients.Disks.BeginGrantAccess(ctx, opts.resourceGroup, opts.diskName, accessReq, nil)
	if err != nil {
		return "", fmt.Errorf("failed to grant disk access: %w", err)
	}

	accessResult, err := accessPoller.PollUntilDone(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get disk access: %w", err)
	}

	uploadURL := *accessResult.AccessSAS

	log.Printf("Uploading VHD to Azure")

	err = uploadVHDToBlob(ctx, vhdPath, uploadURL, verbose)
	if err != nil {
		return "", fmt.Errorf("failed to upload VHD: %w", err)
	}

	if verbose {
		log.Printf("Revoking disk access")
	}

	revokePoller, err := clients.Disks.BeginRevokeAccess(ctx, opts.resourceGroup, opts.diskName, nil)
	if err != nil {
		return "", fmt.Errorf("failed to revoke disk access: %w", err)
	}

	_, err = revokePoller.PollUntilDone(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to complete access revocation: %w", err)
	}

	log.Printf("Disk upload completed successfully")

	return diskID, nil
}

func prepareVHDImage(imagePath string, sizeGB int, verbose bool) (string, func(), error) {
	if strings.HasSuffix(strings.ToLower(imagePath), ".vhd") {
		if verbose {
			log.Printf("Input is already a VHD file: %s", imagePath)
		}
		return imagePath, nil, nil
	}

	tempVHD := filepath.Join(os.TempDir(), fmt.Sprintf("upload_%d.vhd", rand.Int()))

	cleanup := func() {
		os.Remove(tempVHD)
	}

	if verbose {
		log.Printf("Converting and resizing image to VHD format")
	}

	resizedPath := tempVHD + ".resized.raw"
	resizeCleanup := func() {
		os.Remove(resizedPath)
	}
	defer resizeCleanup()

	resizeCmd := exec.Command("qemu-img", "resize", imagePath, fmt.Sprintf("%dG", sizeGB))
	if output, err := resizeCmd.CombinedOutput(); err != nil {
		if verbose {
			log.Printf("in-place resize (%q) failed with output %s, moving to a new file\n", resizeCmd.String(), output)
		}
		srcF, err := os.OpenFile(imagePath, os.O_RDWR, 0644)
		if err != nil {
			return "", nil, err
		}
		destF, err := os.OpenFile(resizedPath, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			srcF.Close()
			return "", nil, err
		}
		_, err = io.Copy(srcF, destF)
		srcF.Close()
		destF.Close()
		if err != nil {
			return "", nil, err
		}

		resizeCmd = exec.Command("qemu-img", "resize", resizedPath, fmt.Sprintf("%dG", sizeGB))
		if output, err = resizeCmd.CombinedOutput(); err != nil {
			return "", nil, fmt.Errorf("failed to resize image: %w\noutput: %s", err, output)
		}
		imagePath = resizedPath
	}

	convertCmd := exec.Command("qemu-img", "convert", "-f", "raw", "-O", "vpc",
		"-o", "subformat=fixed,force_size", imagePath, tempVHD)

	if verbose {
		log.Printf("Running: %s", strings.Join(convertCmd.Args, " "))
	}

	if output, err := convertCmd.CombinedOutput(); err != nil {
		return "", nil, fmt.Errorf("failed to convert image to VHD: %w\noutput: %s", err, output)
	}

	return tempVHD, cleanup, nil
}

func getFileSize(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func uploadVHDToBlob(ctx context.Context, vhdPath, uploadURL string, verbose bool) error {
	uploadCmd := exec.Command("azcopy", "copy", vhdPath, uploadURL, "--blob-type=PageBlob")
	if verbose {
		log.Printf("Running: %s\n", uploadCmd.String())
	}
	if output, err := uploadCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to upload disk: %w\noutput: %s", err, output)
	} else if verbose {
		log.Printf("Output: %s\n", output)
	}
	return nil
}

func getExistingDiskID(ctx context.Context, clients *AzureClients, diskName, resourceGroup string) (string, error) {
	disk, err := clients.Disks.Get(ctx, resourceGroup, diskName, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get existing disk: %w", err)
	}

	if disk.ID == nil {
		return "", fmt.Errorf("disk ID is nil")
	}

	return *disk.ID, nil
}

func destroyDisk(ctx context.Context, clients *AzureClients, diskName, resourceGroup string) error {
	if _, err := getExistingDiskID(ctx, clients, diskName, resourceGroup); err != nil {
		return nil
	}
	poller, err := clients.Disks.BeginDelete(ctx, resourceGroup, diskName, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}
