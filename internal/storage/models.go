package storage

// App represents a row in the apps table.
type App struct {
	ID       int64
	BundleID string
	Name     string
	AppType  string
}

// Domain represents a row in the domains table.
type Domain struct {
	ID     int64
	Domain string
}

// Directory represents a row in the directories table.
type Directory struct {
	ID   int64
	Path string
}

// Session represents a row in the sessions table.
type Session struct {
	ID             int64
	AppID          int64
	DomainID       *int64
	DirectoryID    *int64
	StartedAt      int64
	EndedAt        int64
	KeystrokeCount int
	WordCount      int
}

// Word represents a row in the words table.
type Word struct {
	ID          int64
	SessionID   int64
	Word        string
	IsHashed    bool
	TypedAt     int64
	AppID       int64
	DomainID    *int64
	DirectoryID *int64
}
