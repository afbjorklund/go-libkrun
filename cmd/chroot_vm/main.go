/*
 * This is an example implementing chroot-like functionality with libkrun.
 *
 * It executes the requested command (relative to NEWROOT) inside a fresh
 * Virtual Machine created and managed by libkrun.
 */

package main

import (
	"fmt"
	"os"
	"syscall"

	"go-libkrun/pkg/krun"
)

var errno int32

func perror(message string) {
	fmt.Fprintf(os.Stderr, "%s: %d\n", message, errno)
}

func chrootVm(args []string) int {
	var envp = []string{"TEST=works"}
	var portMap = []string{"18000:8000"}
	var rlimits = []string{"6=4096:8192"} // RLIMIT_NPROC = 6
	var rlim syscall.Rlimit

	newRoot := args[1]    // "rootfs_fedora"
	guestArgv := args[2:] // []string{"/bin/sh"}

	// Set the log level to "warn".
	e := krun.SetLogLevel(krun.LogLevelWarn)
	if e != 0 {
		errno = -e
		perror("Error configuring log level")
		return -1
	}

	// Create the configuration context.
	ctx := krun.CreateCtx()
	if ctx < 0 {
		errno = -ctx
		perror("Error creating configuration context")
		return -1
	}
	ctxId := uint32(ctx)

	// Configure the number of vCPUs (1) and the amount of RAM (512 MiB).
	if e := krun.SetVmConfig(ctxId, 4, 4096); e != 0 {
		errno = -e
		perror("Error configuring the number of vCPUs and/or the amount of RAM")
		return -1
	}

	// Raise RLIMIT_NOFILE to the maximum allowed to create some room for virtio-fs
	syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlim)
	rlim.Cur = rlim.Max
	syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rlim)

	if e := krun.SetRoot(ctxId, newRoot); e != 0 {
		errno = -e
		perror("Error configuring root path")
		return -1
	}

	var virglFlags uint32 = krun.VirglrendererUseEgl | krun.VirglrendererDrm |
		krun.VirglrendererThreadSync | krun.VirglrendererUseAsyncFenceCb
	if e := krun.SetGpuOptions(ctxId, virglFlags); e != 0 {
		errno = -e
		perror("Error configuring gpu")
		return -1
	}

	// Map port 18000 in the host to 8000 in the guest
	if e := krun.SetPortMap(ctxId, portMap); e != 0 {
		errno = -e
		perror("Error configuring port map")
		return -1
	}

	// Configure the rlimits that will be set in the guest
	if e := krun.SetRlimits(ctxId, rlimits); e != 0 {
		errno = -e
		perror("Error configuring rlimits")
		return -1
	}

	// Set the working directory to "/", just for the sake of completeness.
	if e := krun.SetWorkdir(ctxId, "/"); e != 0 {
		errno = -e
		perror("Error configuring \"/\" as working directory")
		return -1
	}

	// Specify the path of the binary to be executed in the isolated context, relative to the root path.
	if e := krun.SetExec(ctxId, guestArgv[0], guestArgv[1:], envp); e != 0 {
		errno = -e
		perror("Error configuring the parameters for the executable to be run")
		return -1
	}

	if e := krun.SplitIrqchip(ctxId, false); e != 0 {
		errno = -e
		perror("Error setting split IRQCHIP property")
		return -1
	}

	// Start and enter the microVM. Unless there is some error while creating the microVM
	// this function never returns.
	if e := krun.StartEnter(ctxId); e != 0 {
		errno = -e
		perror("Error creating the microVM")
		return -1
	}

	// Not reached.
	return 0
}

func main() {
	os.Exit(chrootVm(os.Args))
}
