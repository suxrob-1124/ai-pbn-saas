package domainfs

import "time"

type DomainFSContext struct {
	DomainID       string
	DomainURL      string
	DeploymentMode string
	PublishedPath  string
	SiteOwner      string
	ServerID       string
}

type EntryInfo struct {
	Name  string
	IsDir bool
}

type FileInfo struct {
	Path    string
	Name    string
	Size    int64
	IsDir   bool
	ModTime time.Time
}
