package handlers

import (
	"encoding/json"
	"net/http"

	"gorm.io/gorm"

	"pr-reviewer/internal/db/models"
)

type FeedbackHandler struct{ db *gorm.DB }

func NewFeedbackHandler(db *gorm.DB) *FeedbackHandler { return &FeedbackHandler{db: db} }

func (h *FeedbackHandler) Get(w http.ResponseWriter, r *http.Request) {
	commentID, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	user := getUser(r)

	var up, down int64
	h.db.WithContext(r.Context()).Model(&models.CommentFeedback{}).
		Where("review_comment_id = ? AND vote = 1", commentID).Count(&up)
	h.db.WithContext(r.Context()).Model(&models.CommentFeedback{}).
		Where("review_comment_id = ? AND vote = -1", commentID).Count(&down)

	myVote := 0
	if user != nil {
		var fb models.CommentFeedback
		if h.db.WithContext(r.Context()).
			Where("review_comment_id = ? AND user_login = ?", commentID, user.Login).
			First(&fb).Error == nil {
			myVote = fb.Vote
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"up":      up,
		"down":    down,
		"my_vote": myVote,
	})
}

func (h *FeedbackHandler) Submit(w http.ResponseWriter, r *http.Request) {
	commentID, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	user := getUser(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var body struct {
		Vote int `json:"vote"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || (body.Vote != 1 && body.Vote != -1) {
		writeError(w, http.StatusBadRequest, "vote must be 1 or -1")
		return
	}

	var fb models.CommentFeedback
	if h.db.WithContext(r.Context()).
		Where("review_comment_id = ? AND user_login = ?", commentID, user.Login).
		First(&fb).Error == nil {
		h.db.WithContext(r.Context()).Model(&fb).Update("vote", body.Vote)
	} else {
		h.db.WithContext(r.Context()).Create(&models.CommentFeedback{
			ReviewCommentID: commentID,
			UserLogin:       user.Login,
			Vote:            body.Vote,
		})
	}

	var up, down int64
	h.db.WithContext(r.Context()).Model(&models.CommentFeedback{}).
		Where("review_comment_id = ? AND vote = 1", commentID).Count(&up)
	h.db.WithContext(r.Context()).Model(&models.CommentFeedback{}).
		Where("review_comment_id = ? AND vote = -1", commentID).Count(&down)

	writeJSON(w, http.StatusOK, map[string]any{
		"up":      up,
		"down":    down,
		"my_vote": body.Vote,
	})
}
