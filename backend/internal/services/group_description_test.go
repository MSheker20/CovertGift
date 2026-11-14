package services

import (
	"errors"
	"strings"
	"testing"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/google/uuid"
)

type descriptionRepository struct {
	interestsRepository
	saved *string
}

func (r *descriptionRepository) SetDescription(_ uuid.UUID, desc string) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.saved = &desc
	r.group.Description = desc
	return nil
}

func TestNormalizeDescription(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
		err            error
	}{
		{name: "trim", in: "  merhaba \n", want: "merhaba"},
		{name: "inner newlines kept", in: "a\n\nb", want: "a\n\nb"},
		{name: "whitespace only clears", in: " \t\n ", want: ""},
		{name: "empty", in: "", want: ""},
		{name: "1000 runes", in: strings.Repeat("ş", 1000), want: strings.Repeat("ş", 1000)},
		{name: "1000 runes after trim", in: "  " + strings.Repeat("a", 1000) + "  ", want: strings.Repeat("a", 1000)},
		{name: "1001 runes", in: strings.Repeat("ş", 1001), err: ErrInvalidDescription},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeDescription(tc.in)
			if !errors.Is(err, tc.err) || got != tc.want {
				t.Fatalf("got (%q, %v), want (%q, %v)", got, err, tc.want, tc.err)
			}
		})
	}
}

func TestUpdateDescriptionPermissions(t *testing.T) {
	groupID, userID := uuid.New(), uuid.New()
	for _, tc := range []struct {
		name, role string
		target     *uuid.UUID
		noMember   bool
		in         string
		wantErr    error
		wantSaved  bool
	}{
		{name: "admin", role: "admin", in: " yeni ", wantSaved: true},
		{name: "member", role: "member", in: "x", wantErr: ErrUnauthorizedDescription},
		{name: "non member", noMember: true, in: "x", wantErr: ErrNotGroupMember},
		{name: "target", role: "admin", target: &userID, in: "x", wantErr: ErrAntiSpoilerTriggered},
		{name: "too long", role: "admin", in: strings.Repeat("a", 1001), wantErr: ErrInvalidDescription},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &descriptionRepository{interestsRepository: interestsRepository{
				group: &models.Group{ID: groupID, Description: "eski", TargetUserID: tc.target},
			}}
			if !tc.noMember {
				repo.member = &models.GroupMember{GroupID: groupID, UserID: userID, Role: tc.role}
			}
			svc := NewGroupService(repo, nil, nil)
			group, err := svc.UpdateDescription(groupID, userID, tc.in)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if (repo.saved != nil) != tc.wantSaved {
				t.Fatalf("saved = %v, want %v", repo.saved != nil, tc.wantSaved)
			}
			if tc.wantSaved && (group == nil || group.Description != "yeni") {
				t.Fatalf("güncel grup dönmedi: %+v", group)
			}
		})
	}
}

func TestTargetAccountIsMinimal(t *testing.T) {
	id := uuid.New()
	if targetAccount(&models.Group{}) != nil {
		t.Fatal("hedefsiz grupta özet döndü")
	}
	if targetAccount(&models.Group{TargetUserID: &id}) != nil {
		t.Fatal("hesap yüklenmeden özet döndü")
	}
	got := targetAccount(&models.Group{TargetUserID: &id, TargetUser: &models.User{ID: id, Name: "Ayşe", Email: "ayse@demo.com", PasswordHash: "x"}})
	if got == nil || got.ID != id || got.Name != "Ayşe" {
		t.Fatalf("özet yanlış: %+v", got)
	}
}
