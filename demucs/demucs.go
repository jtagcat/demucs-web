package demucs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

var (
	Models = map[string]struct {
		Fono    []string
		NonFono []string
		jobs    int
	}{
		"mdx_extra":   {jobs: intEnvDefault("JOBS_MDX_EXTRA", 16), Fono: []string{"bass", "drums", "other"}, NonFono: []string{"vocals"}},
		"htdemucs":    {jobs: intEnvDefault("JOBS_HTDEMUCS", 8), Fono: []string{"bass", "drums", "other"}, NonFono: []string{"vocals"}},
		"htdemucs_ft": {jobs: intEnvDefault("JOBS_HTDEMUCS_FT", 8), Fono: []string{"bass", "drums", "other"}, NonFono: []string{"vocals"}},
		"htdemucs_6s": {jobs: intEnvDefault("JOBS_HTDEMUCS_6S", 8), Fono: []string{"bass", "drums", "guitar", "piano", "other"}, NonFono: []string{"vocals"}},
		"hdemucs_mmi": {jobs: intEnvDefault("JOBS_HDEMUCS_MMI", 8), Fono: []string{"bass", "drums", "other"}, NonFono: []string{"vocals"}},
	}

	ModelNames []string = func() (names []string) {
		for model := range Models {
			names = append(names, model)
		}

		return
	}()

	// for UI
	SampleOrder = func() map[string]int {
		ordered := make(map[string]int)

		for i, key := range []string{"original", "fono", "vocals", "drums", "bass", "guitar", "piano", "other"} {
			ordered[key] = i
		}

		return ordered
	}()
)

// stems: map[stem]path
func Split(ctx context.Context, model string, overrideJobs int, name, tempDir, targetDir string) (stems map[string]string, err error) {
	stems = make(map[string]string)

	tempDir, terr := os.MkdirTemp(tempDir, "demucs")
	if terr != nil {
		return nil, fmt.Errorf("creating temporary directory: %w", terr)
	}

	defer func() {
		errDefer := os.RemoveAll(tempDir)
		if err == nil && errDefer != nil {
			err = fmt.Errorf("removing temporary directory: %w", errDefer)
		}
	}()

	modelStems := append(Models[model].Fono, Models[model].NonFono...)

	jobs := Models[model].jobs
	if overrideJobs > 0 {
		jobs = overrideJobs
	}

	cmd := exec.CommandContext(ctx,
		"demucs",
		"-n", model,
		"--jobs", fmt.Sprint(jobs),
		"--filename", "{stem}-{track}.{ext}",
		"-o", tempDir,
		"--mp3",
		"--", name,
	)

	stdmix := new(strings.Builder)
	cmd.Stdout, cmd.Stderr = stdmix, stdmix

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("executing demucs: %w: %s", err, stdmix.String())
	}

	outputDir := path.Join(tempDir, model)
	outDir, err := os.ReadDir(outputDir)
	if err != nil {
		return nil, fmt.Errorf("reading temporary output directory: %w", err)
	}

	for _, file := range outDir {
		stem, _, ok := strings.Cut(file.Name(), "-")
		if !ok {
			return nil, fmt.Errorf("unexpected file name %q in temporary output directory", file.Name())
		}

		if ok := inSlice(stem, modelStems); !ok {
			return nil, fmt.Errorf("unexpected stem name %s in model %s", stem, model)
		}

		newPath := path.Join(targetDir, file.Name())
		if err := os.Rename(path.Join(outputDir, file.Name()), newPath); err != nil {
			return nil, fmt.Errorf("moving stem file: %w", err)
		}

		stems[stem] = newPath
	}

	if len(stems) != len(modelStems) {
		return nil, fmt.Errorf("expected %d stems %v, got %v", len(modelStems), modelStems, stems)
	}

	return
}

func inSlice[T comparable](s T, sl []T) bool {
	for _, slItem := range sl {
		if s == slItem {
			return true
		}
	}

	return false
}

func intEnvDefault(name string, defaul int) int {
	env := os.Getenv(name)
	if i, err := strconv.Atoi(env); err == nil {
		return i
	}

	return defaul
}
