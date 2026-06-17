package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const driverName = "storage.csi.upclous.com"

func TestSfdiskOutputGetLastPartition(t *testing.T) {
	t.Parallel()
	outputMultiple := `
		Device
		/dev/vda1
		/dev/vda2
		/dev/vda3
	`
	outputSingle := `
		Device
		/dev/vda1
	`
	outputNone := `
		Device
	`
	want := "/dev/vda3"
	got, _ := sfdiskOutputGetLastPartition("/dev/vda", outputMultiple)
	if want != got {
		t.Errorf("sfdiskOutputGetLastPartition failed want %s got %s", want, got)
	}

	want = "/dev/vda1"
	got, _ = sfdiskOutputGetLastPartition("/dev/vda", outputSingle)
	if want != got {
		t.Errorf("sfdiskOutputGetLastPartition failed want %s got %s", want, got)
	}

	want = ""
	got, _ = sfdiskOutputGetLastPartition("/dev/vda", outputNone)
	if want != got {
		t.Errorf("sfdiskOutputGetLastPartition failed want %s got %s", want, got)
	}
}

func TestLinuxFilesystem_Mount(t *testing.T) {
	t.Parallel()
	if err := checkSystemRequirements(); err != nil {
		t.Skipf("skipping test: %s", err.Error())
	}
	// create 10MB fake partition
	part, err := createDeviceFile(1e7)
	if err != nil {
		t.Error(err)
		return
	}
	defer os.Remove(part)
	t.Logf("create fake partition %s", part)

	m := newTestLinuxFilesystem()

	if err := m.createFilesystemIfNotExists(context.Background(), part, "ext4", nil); err != nil {
		t.Errorf("Format failed with error: %s", err.Error())
		return
	}
	t.Logf("formated %s", part)
	s, err := m.Statistics(os.TempDir())
	if err != nil {
		t.Errorf("GetStatistics failed with error: %s", err.Error())
		return
	}
	t.Logf("got %s statistics", os.TempDir())
	if s.AvailableBytes <= 0 {
		t.Errorf("GetStatistics failed available bytes if zero")
		return
	}

	if canMount() {
		if err := mountFilesystem(t, m, part); err != nil {
			t.Error(err)
			return
		}
		if err := mountBlockDevice(t, m, part); err != nil {
			t.Error(err)
			return
		}
	} else {
		t.Log("skipped mount testing")
	}
}

func TestLinuxFilesystem_CreateAndReadPartition(t *testing.T) {
	t.Parallel()
	if err := checkSystemRequirements(); err != nil {
		t.Skipf("skipping test: %s", err.Error())
	}
	// create 10MB fake disk
	disk, err := createDeviceFile(1e7)
	if err != nil {
		t.Error(err)
		return
	}
	defer os.Remove(disk)
	t.Logf("create fake disk device %s", disk)
	m := newTestLinuxFilesystem()

	ctx := context.Background()
	// Create partition table
	if err := m.createPartitionTableIfNotExists(ctx, disk); err != nil {
		t.Errorf("createPartitionTableIfNotExists failed with error: %s", err.Error())
		return
	}

	// check last partition
	wantPartition := disk + "p1"

	// Create partition equivalent to creating /dev/sda1 to device /dev/sda
	lastPartition, err := m.createPartitionIfNotExists(ctx, disk)
	if err != nil {
		t.Errorf("createPartition failed with error: %s", err.Error())
		return
	}
	if wantPartition != lastPartition {
		t.Errorf("createPartition returned unexpeted partition, want %s got %s", wantPartition, lastPartition)
		return
	}

	gotPartition, err := m.GetDeviceLastPartition(context.Background(), disk)
	if err != nil {
		t.Errorf("getLastPartition failed with error: %s", err.Error())
		return
	}
	if wantPartition != gotPartition {
		t.Errorf("getLastPartition failed want %s got %s", wantPartition, gotPartition)
		return
	}
	t.Logf("created new partition %s", wantPartition)
}

func TestVolumeIDToDiskID(t *testing.T) {
	t.Parallel()
	volID := "f67db1ca-825b-40aa-a6f4-390ac6ff1b91"
	want := "virtio-f67db1ca825b40aaa6f4"
	got, err := volumeIDToDiskID(volID)
	require.NoError(t, err)
	if want != got {
		t.Errorf("volumeIDToDiskID('%s') failed want %s got %s", volID, want, got)
	}
}

func TestGetBlockDeviceByDiskID(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp(os.TempDir(), fmt.Sprintf("test-%s-*", driverName))
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	t.Logf("using temp dir %s", tempDir)

	tempDevPath := filepath.Join(tempDir, "dev")
	t.Logf("using dev path %s", tempDevPath)

	idPath := filepath.Join(tempDir, udevDiskByIDPath)
	t.Logf("using disk id path %s", idPath)

	if err := os.MkdirAll(idPath, 0o750); err != nil {
		t.Fatal(err)
	}

	// Test relative path
	vda, err := createTempFile(tempDevPath, "vda")
	require.NoError(t, err)

	vdaUUID := uuid.NewString()
	diskID, err := volumeIDToDiskID(vdaUUID)
	require.NoError(t, err)

	vdaSymLink := filepath.Join(idPath, diskID)

	// using ln command instead of Go's built-in so that link has relative path
	if err := exec.Command("ln", "-s", fmt.Sprintf("../../%s", filepath.Base(vda)), vdaSymLink).Run(); err != nil { //nolint:gosec // test, creates relative symlink
		t.Fatal(err)
	}

	want := vda
	got, err := getBlockDeviceByDiskID(context.TODO(), vdaSymLink)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	// Test absolute path
	vdb, _ := createTempFile(tempDevPath, "vdb")
	vdbUUID := uuid.NewString()
	diskID, err = volumeIDToDiskID(vdbUUID)
	require.NoError(t, err)
	vdbSymLink := filepath.Join(idPath, diskID)
	if err := os.Symlink(vdb, vdbSymLink); err != nil {
		t.Fatal(err)
	}
	want = vdb
	got, err = getBlockDeviceByDiskID(context.TODO(), vdbSymLink)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestLinuxFilesystem_FormatValidation(t *testing.T) {
	t.Parallel()
	fs := newTestLinuxFilesystem()
	require.ErrorContains(t, fs.Format(context.TODO(), "/foo", "ext5", nil), "filesystem type 'ext5' is not supported")
	require.ErrorContains(t, fs.Format(context.TODO(), "/foo", "", nil), "fs type is not specified for formatting the volume")
	require.ErrorContains(t, fs.Format(context.TODO(), "", "ext4", nil), "source is not specified for formatting the volume")
}

func TestLinuxFilesystem_isSupportedFilesystem(t *testing.T) {
	t.Parallel()
	fs := newTestLinuxFilesystem()
	require.NoError(t, fs.isSupportedFilesystem("ext4"))
	require.Error(t, fs.isSupportedFilesystem("extX"))
}

func TestLinuxFilesystem_ResizeVolume(t *testing.T) {
	t.Parallel()
	if err := checkSystemRequirements(); err != nil {
		t.Skipf("skipping test: %s", err.Error())
	}
	if os.Getuid() != 0 {
		t.Skip("skipping test: requires root for loop device and mount")
	}

	tests := []struct {
		name       string
		fsType     string
		needsFsck  bool
		mkfsTool   string
		resizeTool string
		minSize    int64
	}{
		{
			name:       "ext4",
			fsType:     "ext4",
			needsFsck:  true,
			mkfsTool:   "mkfs.ext4",
			resizeTool: "resize2fs",
			minSize:    16 * 1024 * 1024,
		},
		{
			name:       "xfs",
			fsType:     "xfs",
			needsFsck:  false,
			mkfsTool:   "mkfs.xfs",
			resizeTool: "xfs_growfs",
			minSize:    512 * 1024 * 1024,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.fsType, func(t *testing.T) {
			t.Parallel()
			for _, tool := range []string{tt.mkfsTool, tt.resizeTool} {
				if _, err := exec.LookPath(tool); err != nil {
					t.Skipf("skipping %s: %s not found in $PATH", tt.fsType, tool)
				}
			}

			initialSize := 2 * tt.minSize
			resizedSize := 4 * tt.minSize

			disk, err := createDeviceFile(initialSize)
			require.NoError(t, err)
			defer os.Remove(disk)
			t.Logf("created backing file %s (%d bytes)", disk, initialSize)

			loopDev, err := attachLoopDevice(disk)
			require.NoError(t, err)
			defer detachLoopDevice(loopDev)
			t.Logf("attached to loop device %s", loopDev)

			logger := logrus.New()
			logger.SetOutput(io.Discard)
			fs, _ := NewLinuxFilesystem([]string{tt.fsType}, logger.WithFields(nil))

			ctx := context.Background()

			require.NoError(t, fs.Format(ctx, loopDev, tt.fsType, nil))
			t.Logf("formatted %s as %s", loopDev, tt.fsType)

			partition, err := fs.GetDeviceLastPartition(ctx, loopDev)
			require.NoError(t, err)
			t.Logf("got partition %s", partition)

			mountPath, err := os.MkdirTemp(os.TempDir(), fmt.Sprintf("%s-mount-*", driverName))
			require.NoError(t, err)
			defer os.RemoveAll(mountPath)

			require.NoError(t, fs.Mount(ctx, partition, mountPath, tt.fsType))
			t.Logf("mounted %s to %s", partition, mountPath)

			origStats, err := fs.Statistics(mountPath)
			require.NoError(t, err)
			t.Logf("original volume stats: total=%d available=%d", origStats.TotalBytes, origStats.AvailableBytes)

			require.NoError(t, fs.Unmount(ctx, mountPath))
			t.Logf("unmounted %s", mountPath)

			require.NoError(t, os.Truncate(disk, resizedSize))
			require.NoError(t, rescanLoopDevice(loopDev))
			t.Logf("grew backing file to %d bytes and rescanned loop device", resizedSize)

			require.NoError(t, sfdiskResizePartitionFill(loopDev, 1))
			require.NoError(t, reloadPartitionTable(loopDev))
			t.Logf("partition table reloaded for %s", loopDev)

			if tt.needsFsck {
				require.NoError(t, e2fsckForcePartition(partition))
			}

			switch tt.fsType {
			case "ext4":
				// resize2fs works on the block device offline, and parted -s resizepart also needs the partition unused.
				require.NoError(t, fs.ResizeVolume(ctx, loopDev, mountPath))
				t.Log("resized volume (partition + filesystem)")

				require.NoError(t, fs.Mount(ctx, partition, mountPath, tt.fsType))
				defer fs.Unmount(ctx, mountPath)
				t.Logf("mounted %s to %s", partition, mountPath)

			case "xfs":
				// parted's resizepart -s refuses to work on in-use loop-device partitions, but sfdisk already resized
				// the partition. Call the fs-specific resize directly while the filesystem is mounted (xfs_growfs needs a live mount point).
				require.NoError(t, fs.Mount(ctx, partition, mountPath, tt.fsType))
				defer fs.Unmount(ctx, mountPath)
				t.Logf("mounted %s to %s", partition, mountPath)

				fsType, err := detectFilesystemType(ctx, logger.WithField("test", ""), partition)
				require.NoError(t, err)
				require.Equal(t, tt.fsType, fsType)

				require.NoError(t, resizeXfsFilesystem(ctx, logger.WithField("test", ""), mountPath))
				t.Log("resized filesystem")
			}

			newStats, err := fs.Statistics(mountPath)
			require.NoError(t, err)
			t.Logf("new volume stats: total=%d available=%d", newStats.TotalBytes, newStats.AvailableBytes)

			assert.Greater(t, newStats.TotalBytes, origStats.TotalBytes,
				"filesystem total bytes should have increased after resize")
			assert.Greater(t, newStats.AvailableBytes, origStats.AvailableBytes,
				"filesystem available bytes should have increased after resize")
		})
	}
}

func createDeviceFile(size int64) (string, error) {
	f, err := os.CreateTemp(os.TempDir(), fmt.Sprintf("%s-disk-*", driverName))
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := f.Truncate(size); err != nil {
		return f.Name(), err
	}
	return f.Name(), err
}

func checkSystemRequirements() error {
	tools := []string{
		"mkfs.ext4", "mount", "umount", "blkid", "wipefs", "findmnt", "parted", "sfdisk", "tune2fs", "udevadm", "losetup", "resize2fs", "e2fsck",
	}
	for _, t := range tools {
		if _, err := exec.LookPath(t); err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				return fmt.Errorf("%s executable not found in $PATH", t)
			}
			return err
		}
	}
	return nil
}

func newTestLinuxFilesystem() *LinuxFilesystem {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	fs, _ := NewLinuxFilesystem([]string{"ext4"}, logger.WithFields(nil))
	return fs
}

func canMount() bool {
	return os.Getuid() == 0
}

func createTempFile(dir, pattern string) (string, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	return f.Name(), f.Close()
}

func attachLoopDevice(file string) (string, error) {
	cmd := exec.Command("losetup", "--find", "--show", file) //nolint:gosec // test helper, fixed args
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("losetup --find --show %s failed: %w (%s)", file, err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

func detachLoopDevice(device string) error {
	return exec.Command("losetup", "-d", device).Run() //nolint:gosec // test helper, fixed args
}

func rescanLoopDevice(device string) error {
	out, err := exec.Command("losetup", "-c", device).CombinedOutput() //nolint:gosec // test helper, fixed args
	if err != nil {
		return fmt.Errorf("losetup -c %s failed: %w (%s)", device, err, string(out))
	}
	return nil
}

func sfdiskResizePartitionFill(device string, partNum int) error {
	// sfdisk resizes the partition to fill the available space and also corrects the GPT backup header location.  This is the equivalent
	// of "parted -s DEV resizepart N 100%" but sfdisk handles the GPT relocation automatically without prompting.
	//
	// The ", +" input tells sfdisk to set the start sector to the current value (unchanged) and the size to "+" (fill all remaining space).
	cmd := exec.Command("sfdisk", "-q", "-N", fmt.Sprintf("%d", partNum), device) //nolint:gosec // test helper, fixed args
	cmd.Stdin = strings.NewReader(", +\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sfdisk resize partition %s #%d failed: %w (%s)", device, partNum, err, string(out))
	}
	return nil
}

func reloadPartitionTable(device string) error {
	out, err := exec.Command("partx", "-u", device).CombinedOutput() //nolint:gosec // test helper, fixed args
	if err != nil {
		return fmt.Errorf("partx -u %s failed: %w (%s)", device, err, string(out))
	}
	return nil
}

func e2fsckForcePartition(partition string) error {
	out, err := exec.Command("e2fsck", "-fy", partition).CombinedOutput() //nolint:gosec // test helper, fixed args
	if err != nil {
		return fmt.Errorf("e2fsck -f %s failed: %w (%s)", partition, err, string(out))
	}
	return nil
}

func mountFilesystem(t *testing.T, m *LinuxFilesystem, partition string) error {
	t.Helper()
	mountPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s-mount-path-%d", driverName, time.Now().Unix()))
	defer os.RemoveAll(mountPath)

	return mount(t, m, partition, mountPath, "ext4")
}

func mountBlockDevice(t *testing.T, m *LinuxFilesystem, partition string) error {
	t.Helper()
	mountPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s-mount-path-%d", driverName, time.Now().Unix()))
	defer os.RemoveAll(mountPath)

	return mount(t, m, partition, mountPath, "", "bind")
}

func mount(t *testing.T, m *LinuxFilesystem, source, target, fsType string, opts ...string) error {
	t.Helper()

	if err := m.Mount(context.Background(), source, target, fsType, opts...); err != nil {
		return fmt.Errorf("Mount %s %s => %s failed with error: %w", fsType, source, target, err)
	}
	isMounted, err := m.IsMounted(context.Background(), target)
	if err != nil {
		return fmt.Errorf("IsMounted failed with error: %w", err)
	}
	if !isMounted {
		return errors.New("IsMounted returned false")
	}

	t.Logf("mounted %s to %s", source, target)
	if err := m.Unmount(context.Background(), target); err != nil {
		return fmt.Errorf("Unmount failed with error: %w", err)
	}
	t.Logf("unmounted %s", target)
	return nil
}
