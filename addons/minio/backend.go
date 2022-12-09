// Copyright 2020-2022 Danny Liu.

package minio

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"nvr"
	"nvr/pkg/log"
	"nvr/pkg/monitor"
	"nvr/pkg/storage"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var MinioClient *minio.Client
var MINIOENDPOINT string
var MINIOACCESSKEYID string
var MINIOSECRETACCESSKEY string
var MINIOLOCATION string
var MINIOEVENTBUCKET string
var MINIOUSESSL bool

func init() {
	godotenv.Load()
	loadEnv()
	nvr.RegisterLogSource([]string{"minio"})
	nvr.RegisterMonitorRecSavedHook(onRecSaved)
}

func loadEnv() {
	if os.Getenv("MINIOENDPOINT") == "" {
		MINIOENDPOINT = "localhost:9000"
	} else {
		MINIOENDPOINT = os.Getenv("MINIOENDPOINT")
	}

	if os.Getenv("MINIOACCESSKEYID") == "" {
		MINIOACCESSKEYID = "minioadmin"
	} else {
		MINIOACCESSKEYID = os.Getenv("MINIOACCESSKEYID")
	}

	if os.Getenv("MINIOSECRETACCESSKEY") == "" {
		MINIOSECRETACCESSKEY = "minioadmin"
	} else {
		MINIOSECRETACCESSKEY = os.Getenv("MINIOSECRETACCESSKEY")
	}

	if os.Getenv("MINIOLOCATION") == "" {
		MINIOLOCATION = "us-west-1"
	} else {
		MINIOLOCATION = os.Getenv("MINIOLOCATION")
	}
	if os.Getenv("MINIOEVENTBUCKET") == "" {
		MINIOEVENTBUCKET = "testbucket"
	} else {
		MINIOEVENTBUCKET = os.Getenv("MINIOEVENTBUCKET")
	}
	if os.Getenv("MINIOENDPOINT") == "" {
		MINIOUSESSL = false
	} else {
		boolValue, err := strconv.ParseBool(os.Getenv("MINIOUSESSL"))
		if err != nil {
			MINIOUSESSL = false
		}
		MINIOUSESSL = boolValue
	}
}

func onRecSaved(r *monitor.Recorder, recPath string, recData storage.RecordingData) {

	id := r.Config.ID()
	logf := func(level log.Level, format string, a ...interface{}) {
		r.Logger.Log(log.Entry{
			Level:     level,
			Src:       "minio",
			MonitorID: id,
			Msg:       fmt.Sprintf(format, a...),
		})
	}

	if MinioClient == nil {
		MinioClient = ConnectMinio()
	}

	Convert(recPath)

	// for instance: 2022-12-08_09-46-05_xg6y2
	inputFile := filepath.Base(recPath)
	inputFileSlice := strings.Split(inputFile, "_")
	dateStr := inputFileSlice[0]

	// outputPath is like a tag with the file on MinIO
	// for instance: recordings/2022/12/08/2022-12-08_09-46-05_xg6y2.mp4
	// It will be put inside the recordings/2022/12/08 folder on MinIO
	outputPath := "recordings/" + strings.Replace(dateStr, "-", "/", -1) + "/" + inputFile + ".mp4"
	inputPath := recPath + ".mp4"

	logf(log.LevelDebug, "outputPath:%s\n", outputPath)
	logf(log.LevelDebug, "inputPath:%s\n", inputPath)

	// Use FPutObject to upload the video mp4 file
	// upload the video mp4 file
	contentType := "video/mp4"
	startStr := recData.Start.Format("2006-01-02T15:04:05.999999999-07:00")
	endStr := recData.End.Format("2006-01-02T15:04:05.999999999-07:00")
	n, err := MinioClient.FPutObject(context.Background(),
		"testbucket",
		outputPath,
		inputPath,
		minio.PutObjectOptions{
			UserMetadata: map[string]string{"start": startStr, "end": endStr, "id": id},
			UserTags:     map[string]string{"start": startStr, "end": endStr, "id": id},
			ContentType:  contentType})

	if err != nil {
		//"Upload to minio failed
		logf(log.LevelError, err.Error())
	} else {
		//Remove files
		files, err := filepath.Glob(recPath + ".mp4")
		if err != nil {
			panic(err)
		}
		for _, f := range files {
			if err := os.Remove(f); err != nil {
				panic(err)
			}
		}
		logf(log.LevelInfo, "Successfully uploaded video %v to minio: %v with size: %d",
			inputPath, outputPath, n.Size)
	}
}

func ConnectMinio() *minio.Client {
	// Minio Configuration
	// Initialize Minio client
	minioClient, err := minio.New(MINIOENDPOINT,
		&minio.Options{
			Creds:  credentials.NewStaticV4(MINIOACCESSKEYID, MINIOSECRETACCESSKEY, ""),
			Secure: MINIOUSESSL,
		})
	if err != nil {
		fmt.Printf("Connect to Minio Failed: %s\n", err)
	}

	// Connect to bucket: testbucket
	err = minioClient.MakeBucket(context.Background(),
		MINIOEVENTBUCKET,
		minio.MakeBucketOptions{Region: MINIOLOCATION, ObjectLocking: true})
	if err != nil {
		fmt.Printf("Make Bucket %v: %s\n", MINIOEVENTBUCKET, err)
	} else {
		fmt.Println("Successfully created mybucket.")
	}

	// Set custom policy as public rule
	policy := `{"Version": "2012-10-17","Statement": [{"Action": ["s3:GetObject"],"Effect": "Allow", "Principal": {"AWS": ["*"]},"Resource": ["arn:aws:s3:::*/*"],"Sid": ""}]}`
	minioClient.SetBucketPolicy(context.Background(), MINIOEVENTBUCKET, policy)

	return minioClient
}

// Convert can convert .meta and .mdat into .mp4 file
func Convert(recording string) error {
	video, err := storage.NewVideoReader(recording, nil)
	if err != nil {
		return fmt.Errorf("create video reader: %w", err)
	}
	defer video.Close()

	file, err := os.OpenFile(recording+".mp4", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, video)
	if err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return nil
}

// DownloadRecordingMp4, this approach is not worked well
func DownloadRecordingMp4(port string, recPath string) {

	fullURLFile := fmt.Sprintf("http://localhost:%s/api/recording/video/%s", port, filepath.Base(recPath))
	fmt.Printf("fullURLFile: %s\n", fullURLFile)
	downloadFile := recPath + ".mp4"

	// Create blank file
	file, err := os.Create(downloadFile)
	if err != nil {
		fmt.Printf("Create blank file Failed: %s\n", err)
	}
	client := http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.URL.Opaque = r.URL.Path
			return nil
		},
	}
	// Put content on file
	resp, err := client.Get(fullURLFile)
	if err != nil {
		fmt.Printf("Put content on file Failed: %s\n", err)
	}
	defer resp.Body.Close()

	size, err := io.Copy(file, resp.Body)
	defer file.Close()

	fmt.Printf("Downloaded a file %s with size %d\n", downloadFile, size)
}
