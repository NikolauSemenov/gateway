package note_flow

type CommentsRequest struct {
	Text      string `json:"text"`
	CommentId int    `json:"comment_id"`
	UserName  string `json:"user_name"`
	UserId    int    `json:"user_id"`
}

type NotesResponse struct {
	Id        int                `json:"id"`
	Title     string             `json:"title"`
	Content   string             `json:"content"`
	UserId    int                `json:"user_id"`
	FullName  string             `json:"full_name"`
	CreatedAt string             `json:"created_at"`
	Likes     int                `json:"likes"`
	Comments  *[]CommentsRequest `json:"comments"`
}

type NoteRequest struct {
	Id string `json:"id"`
}

type CreateNoteRequest struct {
	Title    string `json:"title"`
	Content  string `json:"content"`
	FullName string `json:"full_name"`
}

type CreateNoteResponse struct {
	Id int `json:"id"`
}

type LikeNoteRequest struct {
	NoteID int `json:"note_id"`
}

type LikeNoteResponse struct {
	Id int `json:"id"`
}

type NoteLikesCountResponse struct {
	Likes int `json:"likes"`
}

type NoteCommentsResponse struct {
	NoteID int `json:"Id"`
}

type NoteCommentsRequest struct {
	NoteID   int    `json:"note_id"`
	FullName string `json:"user_full_name"`
	Text     string `json:"text"`
}
