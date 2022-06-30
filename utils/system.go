package utils

import "os/exec"

func HasFFmpeg() bool {
	return exec.Command("ffmpeg", "-h").Run() == nil
}
