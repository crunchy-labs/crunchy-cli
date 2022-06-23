package crunchyroll

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Comment struct {
	crunchy *Crunchyroll

	EpisodeID string `json:"episode_id"`

	CommentID string `json:"comment_id"`
	DomainID  string `json:"domain_id"`

	GuestbookKey string `json:"guestbook_key"`

	User struct {
		UserKey        string `json:"user_key"`
		UserAttributes struct {
			Username string `json:"username"`
			Avatar   struct {
				Locked   []Image `json:"locked"`
				Unlocked []Image `json:"unlocked"`
			} `json:"avatar"`
		} `json:"user_attributes"`
		UserFlags []any `json:"user_flags"`
	} `json:"user"`

	Message         string `json:"message"`
	ParentCommentID int    `json:"parent_comment_id"`

	Locale LOCALE `json:"locale"`

	UserVotes []string `json:"user_votes"`
	Flags     []string `json:"flags"`
	Votes     struct {
		Inappropriate int `json:"inappropriate"`
		Like          int `json:"like"`
		Spoiler       int `json:"spoiler"`
	} `json:"votes"`

	DeleteReason any `json:"delete_reason"`

	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`

	IsOwner      bool `json:"is_owner"`
	RepliesCount int  `json:"replies_count"`
}

// Delete deleted the current comment. Works only if the user has written the comment.
func (c *Comment) Delete() error {
	if !c.IsOwner {
		return fmt.Errorf("cannot delete, user is not the comment author")
	}
	endpoint := fmt.Sprintf("https://beta.crunchyroll.com/talkbox/guestbooks/%s/comments/%s/flags?locale=%s", c.EpisodeID, c.CommentID, c.crunchy.Locale)
	resp, err := c.crunchy.request(endpoint, http.MethodDelete)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// the api returns a new comment object when modifying it.
	// hopefully this does not change
	json.NewDecoder(resp.Body).Decode(c)

	return nil
}

// IsSpoiler returns if the comment is marked as spoiler or not.
func (c *Comment) IsSpoiler() bool {
	for _, flag := range c.Flags {
		if flag == "spoiler" {
			return true
		}
	}
	return false
}

// MarkAsSpoiler marks the current comment as spoiler. Works only if the user has written the comment,
// and it isn't already marked as spoiler.
func (c *Comment) MarkAsSpoiler() error {
	if !c.IsOwner {
		return fmt.Errorf("cannot mark as spoiler, user is not the comment author")
	} else if c.markedAs("spoiler") {
		return fmt.Errorf("comment is already marked as spoiler")
	}
	endpoint := fmt.Sprintf("https://beta.crunchyroll.com/talkbox/guestbooks/%s/comments/%s/flags?locale=%s", c.EpisodeID, c.CommentID, c.crunchy.Locale)
	body, _ := json.Marshal(map[string][]string{"add": {"spoiler"}})
	req, err := http.NewRequest(http.MethodPatch, endpoint, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	resp, err := c.crunchy.requestFull(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	json.NewDecoder(resp.Body).Decode(c)

	return nil
}

// UnmarkAsSpoiler unmarks the current comment as spoiler. Works only if the user has written the comment,
// and it is already marked as spoiler.
func (c *Comment) UnmarkAsSpoiler() error {
	if !c.IsOwner {
		return fmt.Errorf("cannot mark as spoiler, user is not the comment author")
	} else if !c.markedAs("spoiler") {
		return fmt.Errorf("comment is not marked as spoiler")
	}
	endpoint := fmt.Sprintf("https://beta.crunchyroll.com/talkbox/guestbooks/%s/comments/%s/flags?locale=%s", c.EpisodeID, c.CommentID, c.crunchy.Locale)
	body, _ := json.Marshal(map[string][]string{"remove": {"spoiler"}})
	req, err := http.NewRequest(http.MethodPatch, endpoint, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	resp, err := c.crunchy.requestFull(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	json.NewDecoder(resp.Body).Decode(c)

	return nil
}

// Like likes the comment. Works only if the user hasn't already liked it.
func (c *Comment) Like() error {
	if err := c.vote("like", "liked"); err != nil {
		return err
	}
	c.Votes.Like += 1

	return nil
}

// Liked returns if the user has liked the comment.
func (c *Comment) Liked() bool {
	for _, flag := range c.Flags {
		if flag == "liked" {
			return true
		}
	}
	return false
}

// RemoveLike removes the like from the comment. Works only if the user has liked it.
func (c *Comment) RemoveLike() error {
	if err := c.unVote("like", "liked"); err != nil {
		return err
	}
	c.Votes.Like -= 1

	return nil
}

// Reply replies to the current comment.
func (c *Comment) Reply(message string, spoiler bool) (*Comment, error) {
	endpoint := fmt.Sprintf("https://beta.crunchyroll.com/talkbox/guestbooks/%s/comments?locale=%s", c.EpisodeID, c.crunchy.Locale)
	var flags []string
	if spoiler {
		flags = append(flags, "spoiler")
	}
	body, _ := json.Marshal(map[string]any{"locale": string(c.crunchy.Locale), "message": message, "flags": flags, "parent_id": c.CommentID})
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	resp, err := c.crunchy.requestFull(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	reply := &Comment{}
	if err = json.NewDecoder(resp.Body).Decode(reply); err != nil {
		return nil, err
	}

	return reply, nil
}

// Replies shows all replies to the current comment.
func (c *Comment) Replies(page uint, size uint) ([]*Comment, error) {
	if c.RepliesCount == 0 {
		return []*Comment{}, nil
	}
	endpoint := fmt.Sprintf("https://beta.crunchyroll.com/talkbox/guestbooks/%s/comments/%s/replies?page_size=%d&page=%d&locale=%s", c.EpisodeID, c.CommentID, size, page, c.Locale)
	resp, err := c.crunchy.request(endpoint, http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]any
	json.NewDecoder(resp.Body).Decode(&jsonBody)

	var comments []*Comment
	if err = decodeMapToStruct(jsonBody["items"].([]any), &comments); err != nil {
		return nil, err
	}
	return comments, nil
}

// Report reports the comment. Only works if the comment hasn't been reported yet.
func (c *Comment) Report() error {
	return c.vote("inappropriate", "reported")
}

// RemoveReport removes the report request from the comment. Only works if the user
// has reported the comment.
func (c *Comment) RemoveReport() error {
	return c.unVote("inappropriate", "reported")
}

// FlagAsSpoiler sends a request to the user (and / or crunchyroll?) to mark the comment
// as spoiler. Only works if the comment hasn't been flagged as spoiler yet.
func (c *Comment) FlagAsSpoiler() error {
	return c.vote("spoiler", "spoiler")
}

// UnflagAsSpoiler rewokes the request to the user (and / or crunchyroll?) to mark the
// comment as spoiler. Only works if the user has flagged the comment as spoiler.
func (c *Comment) UnflagAsSpoiler() error {
	return c.unVote("spoiler", "spoiler")
}

func (c *Comment) markedAs(voteType string) bool {
	for _, userVote := range c.UserVotes {
		if userVote == voteType {
			return true
		}
	}
	return false
}

func (c *Comment) vote(voteType, readableName string) error {
	if c.markedAs(voteType) {
		return fmt.Errorf("comment is already marked as %s", readableName)
	}

	endpoint := fmt.Sprintf("https://beta.crunchyroll.com/talkbox/guestbooks/%s/comments/%s/votes?locale=%s", c.EpisodeID, c.CommentID, c.crunchy.Locale)
	body, _ := json.Marshal(map[string]string{"vote_type": voteType})
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	_, err = c.crunchy.requestFull(req)
	if err != nil {
		return err
	}
	c.UserVotes = append(c.UserVotes, voteType)

	return nil
}

func (c *Comment) unVote(voteType, readableName string) error {
	for i, userVote := range c.UserVotes {
		if userVote == voteType {
			endpoint := fmt.Sprintf("https://beta.crunchyroll.com/talkbox/guestbooks/%s/comments/%s/votes?vote_type=%s&locale=%s", c.EpisodeID, c.CommentID, voteType, c.crunchy.Locale)
			_, err := c.crunchy.request(endpoint, http.MethodDelete)
			if err != nil {
				return err
			}
			c.UserVotes = append(c.UserVotes[:i], c.UserVotes[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("comment is not marked as %s", readableName)
}
