package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jtagcat/util/std"
)

func BlendAudio(ctx context.Context, inputs []string, output string) error {
	args := []string{"-hide_banner", "-loglevel", "warning"}
	for _, input := range inputs {
		args = append(args, "-i", input)
	}
	args = append(args, []string{
		"-filter_complex", fmt.Sprintf("amix=dropout_transition=0:normalize=0:inputs=%d", len(inputs)),
		"--", output,
	}...)

	cmd := exec.Command("ffmpeg", args...)

	stdmix := new(strings.Builder)
	cmd.Stdout, cmd.Stderr = stdmix, stdmix

	err := std.RunCmdWithCtx(ctx, cmd)
	if err != nil {
		err = fmt.Errorf("%w: %s", err, stdmix.String())

		_ = os.Remove(output)
	}

	return err
}
