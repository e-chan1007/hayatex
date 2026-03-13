package installer

import "time"

const (
	InstallJobPending   = "pending"
	InstallJobExecuting = "executing"
	InstallJobCompleted = "completed"
)

type InstallJob struct {
	key         string
	Name        string
	Status      string
	Message     string
	LastUpdated time.Time
	HasProgress bool
	Progress    float64
}

func NewInstallJob(key string, name string, hasProgress bool) InstallJob {
	return InstallJob{
		key:         key,
		Name:        name,
		Status:      InstallJobPending,
		Message:     "",
		HasProgress: hasProgress,
		LastUpdated: time.Unix(0, 0),
	}
}

type InstallJobList struct {
	Jobs    []InstallJob
	channel chan<- *InstallJobList
}

func (jl InstallJobList) UpdateJobStatus(jobKey, status string) {
	for i, job := range jl.Jobs {
		if job.key == jobKey {
			jl.Jobs[i].Status = status
			jl.Jobs[i].LastUpdated = time.Now()
			jl.channel <- &jl
			return
		}
	}
}

func (jl InstallJobList) UpdateJobStatusWithMessage(jobKey, status, message string) {
	for i, job := range jl.Jobs {
		if job.key == jobKey {
			jl.Jobs[i].Status = status
			jl.Jobs[i].Message = message
			jl.Jobs[i].LastUpdated = time.Now()
			jl.channel <- &jl
			return
		}
	}
}

func (jl InstallJobList) UpdateJobProgress(jobKey string, progress float64) {
	for i, job := range jl.Jobs {
		if job.key == jobKey {
			jl.Jobs[i].Progress = progress
			jl.Jobs[i].LastUpdated = time.Now()
			jl.channel <- &jl
			return
		}
	}
}

func (jl InstallJobList) AllCompleted() bool {
	for _, job := range jl.Jobs {
		if job.Status != InstallJobCompleted {
			return false
		}
	}
	return true
}
