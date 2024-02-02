package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jtagcat/demucs-web/demucs"
	"github.com/jtagcat/util/wakeup"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/util/wait"
)

type (
	Job struct {
		ID      uint  `gorm:"primaryKey"`
		Created int64 `gorm:"autoCreateTime"`

		Link    string // `gorm:"unique"`
		Model   string
		IsRetry bool // for reducing demucs jobs

		State State `gorm:"index"`

		Name            string
		ErrReason       string
		ProcessDuration time.Duration
	}
	Download struct {
		Job  uint   `gorm:"index"`      // same as Job (dunno how to gorm foreignKey)
		Path string `gorm:"primaryKey"` // uses ID as first item in Path
		Name string // type/kind
	}

	JobWithDL struct {
		Job
		ProcessDurationStr string
		Downloads          []Download
	}
)

const (
	SQLDownloadJob     = "job"
	SQLId              = "id"
	SQLLink            = "link"
	SQLModel           = "model"
	SQLState           = "state"
	SQLName            = "name"
	SQLErrorReason     = "err_reason"
	SQLProcessDuration = "process_duration"

	MaxTitleLen = 80
)

type State uint32 // enum:
const (
	Submitted State = iota
	Processing
	Done
	Errored
)

var (
	rootDir = "data"
	tempDir = path.Join(rootDir, "temp")
)

var downloadDir = path.Join(rootDir, "results")

func cErr(c *gin.Context, code int, err error) {
	c.HTML(code, "error.html", gin.H{
		"err": fmt.Sprintf("%d: %s", code, err.Error()),
	})
	c.Abort()
}

func main() {
	ctx, db := context.Background(), initDB()

	checkDeps()

	_ = os.RemoveAll(tempDir)
	os.MkdirAll(tempDir, os.ModePerm)

	// Worker
	wctx, wake := wakeup.WithWakeup(ctx)
	go processLoop(wctx, db)

	// Server
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.LoadHTMLGlob("templates/*.html")

	router.POST("/poolita", func(c *gin.Context) {
		model, link := c.PostForm("model"), c.PostForm("link")

		if _, ok := demucs.Models[model]; !ok {
			cErr(c, http.StatusBadRequest, fmt.Errorf("model is not in whitelist %v", demucs.ModelNames))
			return
		}

		var Jobs []Job
		if err := db.Find(&Jobs, SQLLink+" = ? AND "+SQLModel+" = ?", link, model).Error; err != nil {
			cErr(c, http.StatusInternalServerError, err)
			return
		}
		if len(Jobs) != 0 {
			cErr(c, http.StatusConflict, fmt.Errorf("duplicate: url-model already exists"))
			return
		}

		if err := db.Create(&Job{
			Link:  link,
			Model: model,
		}).Error; err != nil {
			cErr(c, http.StatusBadRequest, err)
			return
		}
		wake.Wakeup()

		c.Redirect(http.StatusFound, "/")
	})

	router.POST("/uuesti", func(c *gin.Context) {
		id, err := strconv.ParseUint(c.PostForm("id"), 10, 0)
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("could not parse id: %e", err))
		}

		var job Job
		if err := db.First(&job, SQLId, id).Error; err != nil {
			cErr(c, http.StatusInternalServerError, err)
			return
		}

		job.IsRetry = true
		job.State, job.ProcessDuration = Submitted, 0

		if err := db.Save(&job).Error; err != nil {
			cErr(c, http.StatusInternalServerError, err)
			return
		}
		wake.Wakeup()

		c.Redirect(http.StatusFound, "/")
	})

	router.POST("/eemalda", func(c *gin.Context) {
		id, err := strconv.ParseUint(c.PostForm("id"), 10, 0)
		if err != nil {
			cErr(c, http.StatusBadRequest, fmt.Errorf("could not parse id: %w", err))
			return
		}

		jobDir := path.Join(downloadDir, fmt.Sprint(id))

		err = os.RemoveAll(jobDir)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			cErr(c, http.StatusInternalServerError, err)
			return
		}

		if err := db.Delete(&Download{}, SQLDownloadJob+" = ?", id).Error; err != nil {
			cErr(c, http.StatusInternalServerError, err)
			return
		}

		if err := db.Delete(&Job{}, SQLId+" = ?", id).Error; err != nil {
			cErr(c, http.StatusInternalServerError, err)
			return
		}

		c.Redirect(http.StatusFound, "/")
	})

	router.GET("/", func(c *gin.Context) {
		var jobs []Job
		if err := db.Find(&jobs).Error; err != nil {
			cErr(c, http.StatusInternalServerError, err)
			return
		}

		sort.SliceStable(jobs, func(i, j int) bool {
			return jobs[i].Created > jobs[j].Created
		})

		var jobsWithDL []JobWithDL
		for _, j := range jobs {
			var downloads []Download
			if err := db.Where(SQLDownloadJob+" = ?", j.ID).Find(&downloads).Error; err != nil {
				cErr(c, http.StatusInternalServerError, err)
				return
			}

			jobsWithDL = append(jobsWithDL, JobWithDL{
				Job:                j,
				ProcessDurationStr: fmt.Sprintf("%s", j.ProcessDuration.Round(time.Second)),
				Downloads:          downloads,
			})
		}

		c.HTML(http.StatusOK, "index.html", gin.H{
			"headHTML": template.HTML(os.Getenv("HEAD_HTML")),
			"results":  jobsWithDL,
		})
	})

	router.Static("/results", downloadDir)

	router.Static("/laulupeo/assets", "laulupeoassets")
	router.GET("/laulupeo/:id", func(c *gin.Context) {
		id := c.Param("id")

		var job Job
		if err := db.First(&job, SQLId, id).Error; err != nil {
			cErr(c, http.StatusInternalServerError, err)
			return
		}

		var rawDownloads []Download
		if err := db.Where(SQLDownloadJob+" = ?", job.ID).Find(&rawDownloads).Error; err != nil {
			cErr(c, http.StatusInternalServerError, err)
			return
		}

		var downloads []Download
		for _, d := range rawDownloads {
			switch d.Name {
				case "original", "fono": continue
			}

			downloads = append(downloads, d)
		}

		c.HTML(http.StatusOK, "laulupeo.html", gin.H{
			"job": JobWithDL{
				Job:                job,
				ProcessDurationStr: fmt.Sprintf("%s", job.ProcessDuration.Round(time.Second)),
				Downloads:          downloads,
			},
		})
	})

	log.Println("listening on :8080")
	router.Run()
}

func parseCheckbox(val string) bool {
	if val == "on" {
		return true
	}
	return false
}

func initDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open(path.Join(rootDir, "demucs-web.sqlite")), &gorm.Config{})
	if err != nil {
		log.Fatalf("opening database: %e", err)
	}

	if err := db.AutoMigrate(&Job{}, &Download{}); err != nil {
		log.Fatalf("migrating database: %e", err)
	}

	// State: Processing → Submitted
	if err := db.Model(&Job{}).Where(SQLState+" = ?", Processing).Update(SQLState, Submitted).Error; err != nil {
		log.Fatalf("reverting database statuses: %e", err)
	}

	return db
}

func dbBackoff() wait.Backoff {
	return wait.Backoff{
		Duration: time.Second,
		Factor:   4,
		Steps:    3,
	}
}

func checkDeps() {
	if err := exec.Command("yt-dlp", "--version").Run(); err != nil {
		log.Fatalf("running yt-dlp: %e", err)
	}

	if err := exec.Command("demucs", "--help").Run(); err != nil {
		log.Fatalf("running demucs: %e", err)
	}

	if err := exec.Command("ffmpeg", "-version").Run(); err != nil {
		log.Fatalf("running ffmpeg: %e", err)
	}
}
