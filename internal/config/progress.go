package config

import (
	"io"
	"time"
)

type DownloadProgress struct {
	ServiceType string        `json:"serviceType"`
	Version     string        `json:"version"`
	Phase       string        `json:"phase"`
	Progress    int           `json:"progress"`
	BytesRead   int64         `json:"bytesRead"`
	TotalBytes  int64         `json:"totalBytes"`
	Speed       float64       `json:"speed"`
	ETA         time.Duration `json:"eta"`
	StartTime   time.Time     `json:"startTime"`
	LastUpdate  time.Time     `json:"lastUpdate"`
	Error       string        `json:"error,omitempty"`
}

type ProgressReader struct {
	Reader       io.Reader
	Total        int64
	Current      int64
	StartTime    time.Time
	LastReadTime time.Time
	LastBytes    int64
	OnProg       func(int)
	Phase        string
	lastProgress int
}

func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = pr.Reader.Read(p)
	pr.Current += int64(n)

	if pr.Total > 0 && pr.OnProg != nil {
		progress := int(float64(pr.Current) / float64(pr.Total) * 100)

		if progress != pr.lastProgress {
			pr.lastProgress = progress
			pr.OnProg(progress)
		}
	}

	return
}

type progressReader struct {
	Reader       io.Reader
	Total        int64
	Current      int64
	StartTime    time.Time
	LastReadTime time.Time
	LastBytes    int64
	OnProg       func(*DownloadProgress)
	Phase        string
	lastProgress int
}

func (pr *progressReader) Read(p []byte) (n int, err error) {
	n, err = pr.Reader.Read(p)
	pr.Current += int64(n)

	if pr.Total > 0 && pr.OnProg != nil {
		progress := int(float64(pr.Current) / float64(pr.Total) * 100)

		if progress != pr.lastProgress {
			pr.lastProgress = progress

			now := time.Now()
			if !pr.StartTime.IsZero() && !pr.LastReadTime.IsZero() {
				elapsed := now.Sub(pr.LastReadTime).Seconds()
				if elapsed > 0 {
					speed := float64(pr.Current-pr.LastBytes) / elapsed

					eta := time.Duration(0)
					if speed > 0 {
						eta = time.Duration(float64(pr.Total-pr.Current)/speed) * time.Second
					}

					pr.OnProg(&DownloadProgress{
						Progress:   progress,
						BytesRead:  pr.Current,
						TotalBytes: pr.Total,
						Speed:      speed,
						ETA:        eta,
						StartTime:  pr.StartTime,
						LastUpdate: now,
						Phase:      pr.Phase,
					})
				}
			} else if pr.StartTime.IsZero() {
				pr.StartTime = time.Now()
			}

			pr.LastReadTime = now
			pr.LastBytes = pr.Current
		}
	}

	return
}
