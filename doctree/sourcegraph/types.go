package sourcegraph

type LocationResource struct {
	Path       string
	Content    string
	Repository Repository
	Commit     Commit
}

// Zero is the first line. Zero is the first character.
type LineOffset struct {
	Line      uint64
	Character uint64
}

type Range struct {
	Start LineOffset
	End   LineOffset
}

type LocationNode struct {
	URL      string
	Resource LocationResource
	Range    Range
}

type PageInfo struct {
	TotalCount uint64
	PageInfo   struct {
		EndCursor   *string
		HasNextPage bool
	}
}

type Location struct {
	Nodes    []LocationNode
	PageInfo PageInfo
}

type LSIFBlob struct {
	References      []Location
	Implementations []Location
	Definitions     []Location
}

type Blob struct {
	LSIF *LSIFBlob
}

type Commit struct {
	ID   string
	OID  string
	Blob *Blob
}

type Repository struct {
	ID         string
	Name       string
	Stars      uint64
	IsFork     bool
	IsArchived bool
	Commit     *Commit `json:"commit"`
}
