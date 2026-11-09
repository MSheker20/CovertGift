package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type targetGroupService struct {
	services.GroupService
	err         error
	gotEmail    string
	gotTargetID *uuid.UUID
}

func (s *targetGroupService) CreateGroup(_ uuid.UUID, title, _, name, _ string, targetUserID *uuid.UUID, targetEmail string, _ decimal.Decimal, _ *models.Date) (*models.Group, error) {
	s.gotEmail, s.gotTargetID = targetEmail, targetUserID
	if s.err != nil {
		return nil, s.err
	}
	return &models.Group{ID: uuid.New(), Title: title, TargetPersonName: name, TargetUserID: targetUserID}, nil
}

func (s *targetGroupService) LinkTarget(groupID, _ uuid.UUID, email string) (*models.Group, error) {
	s.gotEmail = email
	if s.err != nil {
		return nil, s.err
	}
	id := uuid.New()
	return &models.Group{ID: groupID, TargetUserID: &id}, nil
}

func serveGroup(t *testing.T, svc services.GroupService, method, path, body string) (int, Response) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	controller := NewGroupController(svc)
	withUser := func(handler gin.HandlerFunc) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Set("userID", uuid.New())
			handler(c)
		}
	}
	router.POST("/groups", withUser(controller.CreateGroup))
	router.PATCH("/groups/:id/target", withUser(controller.LinkTarget))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	var response Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("invalid JSON %q: %v", recorder.Body, err)
	}
	return recorder.Code, response
}

func TestCreateGroupTargetEmailResponses(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		err        error
		wantStatus int
		wantEmail  string
	}{
		{name: "email is passed to service", body: `{"title":"Sürpriz","target_person_name":"Ahmet","target_email":"ahmet@demo.com"}`, wantStatus: http.StatusCreated, wantEmail: "ahmet@demo.com"},
		{name: "email is optional", body: `{"title":"Sürpriz","target_person_name":"Ahmet"}`, wantStatus: http.StatusCreated},
		{name: "malformed email rejected before service", body: `{"title":"Sürpriz","target_person_name":"Ahmet","target_email":"not-an-email"}`, wantStatus: http.StatusBadRequest},
		{name: "unregistered email", body: `{"title":"Sürpriz","target_person_name":"Ahmet","target_email":"x@demo.com"}`, err: services.ErrTargetUserNotFound, wantStatus: http.StatusNotFound, wantEmail: "x@demo.com"},
		{name: "self target", body: `{"title":"Sürpriz","target_person_name":"Ahmet","target_email":"me@demo.com"}`, err: services.ErrSelfTarget, wantStatus: http.StatusBadRequest, wantEmail: "me@demo.com"},
		{name: "both target fields", body: `{"title":"Sürpriz","target_person_name":"Ahmet","target_email":"a@demo.com","target_user_id":"` + uuid.NewString() + `"}`, err: services.ErrTargetInputConflict, wantStatus: http.StatusBadRequest, wantEmail: "a@demo.com"},
		{name: "database error hidden", body: `{"title":"Sürpriz","target_person_name":"Ahmet"}`, err: errors.New("pq: secret detail"), wantStatus: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &targetGroupService{err: tc.err}
			status, body := serveGroup(t, svc, http.MethodPost, "/groups", tc.body)
			if status != tc.wantStatus {
				t.Fatalf("status = %d, want %d (%+v)", status, tc.wantStatus, body)
			}
			if body.Success != (status < 300) || strings.Contains(body.Error, "secret detail") {
				t.Fatalf("unexpected envelope: %+v", body)
			}
			if svc.gotEmail != tc.wantEmail {
				t.Fatalf("service email = %q, want %q", svc.gotEmail, tc.wantEmail)
			}
		})
	}
}

func TestLinkTargetResponses(t *testing.T) {
	path := "/groups/" + uuid.NewString() + "/target"
	for _, tc := range []struct {
		name, path, body string
		err              error
		wantStatus       int
	}{
		{name: "linked", body: `{"email":"ahmet@demo.com"}`, wantStatus: http.StatusOK},
		{name: "invalid group id", path: "/groups/nope/target", body: `{"email":"ahmet@demo.com"}`, wantStatus: http.StatusBadRequest},
		{name: "email required", body: `{}`, wantStatus: http.StatusBadRequest},
		{name: "malformed email", body: `{"email":"ahmet"}`, wantStatus: http.StatusBadRequest},
		{name: "member forbidden", body: `{"email":"ahmet@demo.com"}`, err: services.ErrUnauthorizedTargetUpdate, wantStatus: http.StatusForbidden},
		{name: "anti spoiler", body: `{"email":"ahmet@demo.com"}`, err: services.ErrAntiSpoilerTriggered, wantStatus: http.StatusForbidden},
		{name: "outsider", body: `{"email":"ahmet@demo.com"}`, err: services.ErrNotGroupMember, wantStatus: http.StatusForbidden},
		{name: "group missing", body: `{"email":"ahmet@demo.com"}`, err: services.ErrGroupNotFound, wantStatus: http.StatusNotFound},
		{name: "unregistered email", body: `{"email":"ahmet@demo.com"}`, err: services.ErrTargetUserNotFound, wantStatus: http.StatusNotFound},
		{name: "self target", body: `{"email":"ahmet@demo.com"}`, err: services.ErrSelfTarget, wantStatus: http.StatusBadRequest},
		{name: "already linked", body: `{"email":"ahmet@demo.com"}`, err: services.ErrTargetAlreadyLinked, wantStatus: http.StatusConflict},
		{name: "target is member", body: `{"email":"ahmet@demo.com"}`, err: services.ErrTargetIsMember, wantStatus: http.StatusConflict},
		{name: "database error hidden", body: `{"email":"ahmet@demo.com"}`, err: errors.New("pq: secret detail"), wantStatus: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := path
			if tc.path != "" {
				p = tc.path
			}
			status, body := serveGroup(t, &targetGroupService{err: tc.err}, http.MethodPatch, p, tc.body)
			if status != tc.wantStatus {
				t.Fatalf("status = %d, want %d (%+v)", status, tc.wantStatus, body)
			}
			if body.Success != (status == http.StatusOK) || strings.Contains(body.Error, "secret detail") {
				t.Fatalf("unexpected envelope: %+v", body)
			}
		})
	}
}
