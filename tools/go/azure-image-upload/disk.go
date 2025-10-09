package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/pageblob"
)

func uploadDiskImage(ctx context.Context, clients *AzureClients, imagePath, diskName string, sizeGB int, resourceGroup, region string, arch armcompute.Architecture, tags map[string]*string, uploadJobs int, verbose bool) (string, error) {
	vhdPath, cleanup, err := prepareVHDImage(imagePath, sizeGB, verbose)
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
		Location: to.Ptr(region),
		Properties: &armcompute.DiskProperties{
			CreationData: &armcompute.CreationData{
				CreateOption:    to.Ptr(armcompute.DiskCreateOptionUpload),
				UploadSizeBytes: to.Ptr(int64(vhdSize)),
			},
			OSType:           to.Ptr(armcompute.OperatingSystemTypesLinux),
			HyperVGeneration: to.Ptr(armcompute.HyperVGenerationV2),
		},
		SKU: &armcompute.DiskSKU{
			Name: to.Ptr(armcompute.DiskStorageAccountTypesStandardLRS),
		},
		Tags: tags,
	}

	if verbose {
		log.Printf("Creating disk: %s", diskName)
	}

	poller, err := clients.Disks.BeginCreateOrUpdate(ctx, resourceGroup, diskName, disk, nil)
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

	if verbose {
		log.Printf("Granting write access to disk")
	}

	accessPoller, err := clients.Disks.BeginGrantAccess(ctx, resourceGroup, diskName, accessReq, nil)
	if err != nil {
		return "", fmt.Errorf("failed to grant disk access: %w", err)
	}

	accessResult, err := accessPoller.PollUntilDone(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get disk access: %w", err)
	}

	uploadURL := *accessResult.AccessSAS

	if verbose {
		log.Printf("Uploading VHD to Azure")
	}

	err = uploadVHDToBlob(ctx, vhdPath, uploadURL, uploadJobs, verbose)
	if err != nil {
		return "", fmt.Errorf("failed to upload VHD: %w", err)
	}

	if verbose {
		log.Printf("Revoking disk access")
	}

	revokePoller, err := clients.Disks.BeginRevokeAccess(ctx, resourceGroup, diskName, nil)
	if err != nil {
		return "", fmt.Errorf("failed to revoke disk access: %w", err)
	}

	_, err = revokePoller.PollUntilDone(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to complete access revocation: %w", err)
	}

	if verbose {
		log.Printf("Disk upload completed successfully")
	}

	return diskID, nil
}

func prepareVHDImage(imagePath string, sizeGB int, verbose bool) (string, func(), error) {
	if strings.HasSuffix(strings.ToLower(imagePath), ".vhd") {
		if verbose {
			log.Printf("Input is already a VHD file: %s", imagePath)
		}
		return imagePath, nil, nil
	}

	tempDir := os.TempDir()
	tempVHD := filepath.Join(tempDir, fmt.Sprintf("upload_%d.vhd", time.Now().Unix()))

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

func uploadVHDToBlob(ctx context.Context, vhdPath, uploadURL string, jobs int, verbose bool) error {
	file, err := os.Open(vhdPath)
	if err != nil {
		return fmt.Errorf("failed to open VHD file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat VHD file: %w", err)
	}
	fileSize := stat.Size()

	client, err := pageblob.NewClientWithNoCredential(uploadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create blob client: %w", err)
	}

	// Page blobs require 512-byte aligned chunks,
	// with a maximum of 4MiB per chunk.
	const chunkSize = 4 * 1024 * 1024
	var offset, uploaded int64 = 0, 0
	var wg sync.WaitGroup

	// Hold this to read or write to offset or file.
	var vhdMu sync.Mutex

	var uploadedMu sync.Mutex

	var errs []error
	var errsMu sync.RWMutex

	uploadStart := time.Now()

	logErr := func(e error) {
		errsMu.Lock()
		errs = append(errs, e)
		if verbose {
			log.Printf("error during VHD upload, canceling\n")
		}
	}

	logUpload := func(uploaded int64) {
		elapsed := time.Since(uploadStart)
		avgSpeedMBps := float64(uploaded) / (1024 * 1024) / elapsed.Seconds()
		log.Printf("Uploaded %d MB / %d MB (avg speed: %.2f MB/s)",
			offset/(1024*1024), fileSize/(1024*1024), avgSpeedMBps)
	}

	if verbose {
		log.Printf("uploading VHD with %d jobs\n", jobs)
		logUpload(0)
	}

	// Launch jobs to upload
	for i := 0; i < jobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Get buffer for this job, start upload loop.
			buffer := make([]byte, chunkSize)
			for {
				// If another job failed, exit early.
				errsMu.RLock()
				done := len(errs) > 0
				errsMu.RUnlock()
				if done {
					break
				}

				// Lock upload resources, then we can read a chunk into our buffer.
				// Release lock if we encounter an error
				vhdMu.Lock()
				n, err := file.Read(buffer)
				if err == io.EOF {
					vhdMu.Unlock()
					break
				}
				if err != nil {
					logErr(fmt.Errorf("failed to read from file: %w", err))
					vhdMu.Unlock()
					break
				}

				if n%512 != 0 {
					// n is some non-integer multiple of 512
					// get that multiple as an int, add 1, mutiply by 512
					// and we're at the next 512-aligned size.
					paddedSize := ((n / 512) + 1) * 512
					for i := n; i < paddedSize; i++ {
						buffer[i] = 0
					}
					n = paddedSize
				}

				thisOffset := offset

				// Bump offset, we're responsible for uploading it now. Unlock resources
				// for next job.
				offset += int64(n)

				vhdMu.Unlock()

				// Upload chunk
				_, err = client.UploadPages(ctx, streaming.NopCloser(bytes.NewReader(buffer[:n])),
					blob.HTTPRange{Offset: thisOffset, Count: int64(n)}, nil)
				if err != nil {
					logErr(fmt.Errorf("failed to upload page at offset %d: %w", thisOffset, err))
					break
				}

				if verbose {
					uploadedMu.Lock()
					uploaded += int64(n)
					if uploaded%(100*1024*1024) == 0 { // Log every 100 MiB
						// Move cursor up one line, clear it, and return to beginning
						fmt.Fprint(os.Stderr, "\033[1A\033[2K\r")
						logUpload(uploaded)
					}
					uploadedMu.Unlock()
				}

			}
		}()
	}
	wg.Wait()

	errsMu.Lock()
	defer errsMu.Unlock()

	if len(errs) > 0 {
		for _, e := range errs {
			log.Printf("%v\n", e)
		}
		return fmt.Errorf("Failed upload with %d errs\n", len(errs))
	} else if verbose {
		elapsed := time.Since(uploadStart)
		avgSpeedMBps := float64(offset) / (1024 * 1024) / elapsed.Seconds()
		log.Printf("Upload completed: %d bytes (avg speed: %.2f MB/s)", offset, avgSpeedMBps)
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
