package api

type DataFile struct {
	Name    string `json:"Name"`
	Content string `json:"Content"`
}

type MessageBody struct {
	Files []DataFile `json:"Files"`
}

type Event struct {
	ID    string     `json:"ID"`
	Files []DataFile `json:"Files"`
}
