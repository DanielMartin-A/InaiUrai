package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/repository"
)

type EmailService struct {
	memberRepo *repository.MemberRepo
	orgRepo    *repository.OrgRepo
	taskMgr    *TaskManager
	domain     string
}

func NewEmailService(domain string, mr *repository.MemberRepo, or *repository.OrgRepo, tm *TaskManager) *EmailService {
	return &EmailService{domain: domain, memberRepo: mr, orgRepo: or, taskMgr: tm}
}

type InboundEmail struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	Text    string `json:"text"`
	HTML    string `json:"html"`
}

func (s *EmailService) ProcessInbound(ctx context.Context, email InboundEmail) {
	go s.handleEmail(email)
}

func (s *EmailService) handleEmail(email InboundEmail) {
	ctx := context.Background()
	senderEmail := extractEmail(email.From)
	if senderEmail == "" {
		return
	}

	member, err := s.memberRepo.GetByEmail(ctx, senderEmail)
	if err != nil || member == nil {
		slog.Info("email from unregistered user", "email", senderEmail)
		return
	}
	org, _ := s.orgRepo.GetByID(ctx, member.OrgID)
	if org == nil {
		return
	}

	body := email.Text
	if body == "" {
		body = stripHTML(email.HTML)
	}
	if body == "" {
		return
	}

	roleHint := extractRoleHint(email.To, s.domain)
	var input string
	if roleHint != "" {
		input = fmt.Sprintf("[Forwarded email to %s]\nSubject: %s\n\n%s", roleHint, email.Subject, body)
	} else {
		input = fmt.Sprintf("[Forwarded email]\nSubject: %s\n\n%s", email.Subject, body)
	}
	if len(input) > 20000 {
		input = input[:20000]
	}

	result, _ := s.taskMgr.ExecuteSoloTask(ctx, org, member, input)
	if result != "" {
		slog.Info("email task sync response", "sender", senderEmail)
	}
}

func extractEmail(from string) string {
	if idx := strings.Index(from, "<"); idx != -1 {
		if end := strings.Index(from[idx:], ">"); end != -1 {
			return strings.TrimSpace(from[idx+1 : idx+end])
		}
	}
	return strings.TrimSpace(from)
}

func extractRoleHint(to, domain string) string {
	for _, addr := range strings.Split(to, ",") {
		addr = strings.TrimSpace(extractEmail(addr))
		if strings.HasSuffix(addr, "@"+domain) {
			local := strings.Split(addr, "@")[0]
			switch local {
			case "tasks", "team", "ai", "help":
				return ""
			default:
				return local
			}
		}
	}
	return ""
}

func stripHTML(html string) string {
	result := html
	for _, tag := range []string{"<br>", "<br/>", "<br />", "</p>", "</div>"} {
		result = strings.ReplaceAll(result, tag, "\n")
	}
	var out strings.Builder
	inTag := false
	for _, r := range result {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			out.WriteRune(r)
		}
	}
	return strings.TrimSpace(out.String())
}
