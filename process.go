package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"sort"
	"strconv"
	"time"

	"github.com/jtagcat/demucs-web/demucs"
	"github.com/jtagcat/demucs-web/ffmpeg"
	"github.com/jtagcat/demucs-web/ytdlp"
	"github.com/jtagcat/util/retry"
	"github.com/jtagcat/util/std"
	"github.com/jtagcat/util/wakeup"
	"gorm.io/gorm"
)

func init() {
	max, _ := strconv.Atoi(os.Getenv("WORKERS"))
	if max < 1 {
		max = 1
	}
	workerLimit = make(chan struct{}, max)

	timeout, _ = time.ParseDuration(os.Getenv("TIMEOUT"))
	if timeout == 0 {
		timeout = time.Hour
	}
}

var (
	workerLimit chan struct{}
	timeout     time.Duration
)

// wctx: wakeup.WithWakeup()
func processLoop(wctx context.Context, db *gorm.DB) {
	err := wakeup.Wait(wctx, func(ctx context.Context) (goToSleep bool) {
		var Jobs []Job

		_ = retry.OnErrorManagedBackoff(ctx, dbBackoff(), func() (retryable bool, _ error) {
			return true, db.Find(&Jobs, SQLState+" = ?", Submitted).Error
		})

		for _, j := range Jobs {
			select {
			case <-ctx.Done():
				return
			case workerLimit <- struct{}{}:
			}

			_ = retry.OnErrorManagedBackoff(ctx, dbBackoff(), func() (retryable bool, _ error) {
				return true, db.Model(&j).Update(SQLState, Processing).Error
			})

			go j.process(ctx, db)
		}

		return true
	})

	if !errors.Is(err, context.Canceled) {
		log.Fatalf("unexpected error in proccess loop's wakeup: %e", err)
	}
}

func (j Job) process(ctx context.Context, db *gorm.DB) {
	defer func() {
		<-workerLimit
	}()

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	// commit early for UI
	go func() {
		for {
			timer := time.NewTimer(10 * time.Second)

			select {
			case <-timer.C:
			case <-ctx.Done():
			}

			_ = db.Model(&j).Update(SQLProcessDuration, time.Since(start)).Error

			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	downloads, err := func() (downloads []Download, _ error) {
		jobDir := path.Join(downloadDir, fmt.Sprint(j.ID))

		err := os.RemoveAll(jobDir)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("clearing job dir: %w", err)
		}

		name, originalPath, err := ytdlp.DownloadMp3(ctx, j.Link, tempDir, jobDir)
		if err != nil {
			return nil, fmt.Errorf("downloading audio with yt-dlp: %w", err)
		}
		j.Name = std.TrimLen(name, MaxTitleLen)
		_ = db.Model(&j).Update(SQLName, j.Name).Error // update early for UI

		var demucsJobsOverride int
		if j.IsRetry && os.Getenv("JOBS_NOGRACEFULRETRY") != "1" {
			demucsJobsOverride = 1
		}

		stems, err := demucs.Split(ctx, j.Model, demucsJobsOverride, originalPath, tempDir, jobDir)
		if err != nil {
			return nil, fmt.Errorf("splitting audio with demucs: %w", err)
		}

		stems["original"] = path.Join(jobDir, "original-"+path.Base(originalPath))
		if err := os.Rename(originalPath, stems["original"]); err != nil {
			return nil, fmt.Errorf("renaming original file: %w", err)
		}

		var fonoPaths []string
		for _, stem := range demucs.Models[j.Model].Fono {
			fonoPaths = append(fonoPaths, stems[stem])
		}

		stems["fono"] = path.Join(jobDir, "fono-"+path.Base(originalPath))
		if err := ffmpeg.BlendAudio(ctx, fonoPaths, stems["fono"]); err != nil {
			return nil, fmt.Errorf("joining to fono: %w", err)
		}

		for stem, sPath := range stems {
			downloads = append(downloads, Download{
				Job:  j.ID,
				Name: stem,
				Path: path.Join(fmt.Sprint(j.ID), path.Base(sPath)),
			})
		}

		return
	}()
	if err != nil {
		_ = retry.OnErrorManagedBackoff(ctx, dbBackoff(), func() (retryable bool, _ error) {
			return true, db.Model(&j).Update(SQLState, Errored).Error
		})
		_ = retry.OnErrorManagedBackoff(ctx, dbBackoff(), func() (retryable bool, _ error) {
			return true, db.Model(&j).Update(SQLErrorReason, err.Error()).Error
		})

		return
	}

	// for UI consistency
	sort.SliceStable(downloads, func(i, j int) bool {
		return demucs.SampleOrder[downloads[i].Name] < demucs.SampleOrder[downloads[j].Name]
	})

	// not using db.CreateInBatches because error handling
	for _, download := range downloads {
		_ = retry.OnErrorManagedBackoff(ctx, dbBackoff(), func() (retryable bool, _ error) {
			return true, db.Create(&download).Error
		})
	}

	j.ProcessDuration = time.Since(start)
	j.State, j.ErrReason = Done, ""
	_ = retry.OnErrorManagedBackoff(ctx, dbBackoff(), func() (retryable bool, _ error) {
		return true, db.Save(&j).Error
	})
}
