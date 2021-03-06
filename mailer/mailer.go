package mailer

import (
	"net/url"
	"path"
	"time"

	"github.com/netlify/gotrue/conf"
	"github.com/netlify/gotrue/models"
	"github.com/netlify/mailme"

	"github.com/badoux/checkmail"
)

const defaultInviteMail = `<h2>You have been invited</h2>

<p>You have been invited to create a user on {{ .SiteURL }}. Follow this link to accept the invite:</p>
<p><a href="{{ .ConfirmationURL }}">Accept the invite</a></p>`

const defaultConfirmationMail = `<h2>Confirm your signup</h2>

<p>Follow this link to confirm your account:</p>
<p><a href="{{ .ConfirmationURL }}">Confirm your email address</a></p>`

const defaultRecoveryMail = `<h2>Reset password</h2>

<p>Follow this link to reset the password for your account:</p>
<p><a href="{{ .ConfirmationURL }}">Reset password</a></p>`

const defaultEmailChangeMail = `<h2>Confirm email address change</h2>

<p>Follow this link to confirm the update of your email address from {{ .Email }} to {{ .NewEmail }}:</p>
<p><a href="{{ .ConfirmationURL }}">Change email address</a></p>`

// Mailer defines the interface a mailer must implement.
type Mailer interface {
	Send(user *models.User, subject, body string, data map[string]interface{}) error
	InviteMail(user *models.User) error
	ConfirmationMail(user *models.User) error
	RecoveryMail(user *models.User) error
	EmailChangeMail(user *models.User) error
	ValidateEmail(email string) error
}

// TemplateMailer will send mail and use templates from the site for easy mail styling
type TemplateMailer struct {
	SiteURL        string
	MemberFolder   string
	Config         *conf.Configuration
	TemplateMailer *mailme.Mailer
}

type noopMailer struct {
}

// MailSubjects holds the subject lines for the emails
type MailSubjects struct {
	ConfirmationMail string
	RecoveryMail     string
}

// NewMailer returns a new gotrue mailer
func NewMailer(conf *conf.Configuration) Mailer {
	if conf.Mailer.Host == "" {
		return &noopMailer{}
	}

	mailConf := conf.Mailer
	return &TemplateMailer{
		SiteURL:      conf.SiteURL,
		MemberFolder: mailConf.MemberFolder,
		Config:       conf,
		TemplateMailer: &mailme.Mailer{
			Host:    conf.Mailer.Host,
			Port:    conf.Mailer.Port,
			User:    conf.Mailer.User,
			Pass:    conf.Mailer.Pass,
			From:    conf.Mailer.AdminEmail,
			BaseURL: conf.SiteURL,
		},
	}
}

// ValidateEmail returns nil if the email is valid,
// otherwise an error indicating the reason it is invalid
func (m TemplateMailer) ValidateEmail(email string) error {
	if err := checkmail.ValidateFormat(email); err != nil {
		return err
	}

	if err := checkmail.ValidateHost(email); err != nil {
		return err
	}

	return nil
}

// InviteMail sends a invite mail to a new user
func (m *TemplateMailer) InviteMail(user *models.User) error {
	url, err := getSiteURL(m.Config.SiteURL, m.Config.Mailer.MemberFolder, "/invite/"+user.ConfirmationToken)
	if err != nil {
		return err
	}
	data := map[string]interface{}{
		"SiteURL":         m.Config.SiteURL,
		"ConfirmationURL": url,
		"Email":           user.Email,
		"Token":           user.ConfirmationToken,
		"Data":            user.UserMetaData,
	}

	return m.TemplateMailer.Mail(
		user.Email,
		withDefault(m.Config.Mailer.Subjects.Invite, "You have been invited"),
		m.Config.Mailer.Templates.Invite,
		defaultInviteMail,
		data,
	)
}

// ConfirmationMail sends a signup confirmation mail to a new user
func (m *TemplateMailer) ConfirmationMail(user *models.User) error {
	if !user.ConfirmationSentAt.Add(m.Config.Mailer.MaxFrequency).Before(time.Now()) {
		return nil
	}

	url, err := getSiteURL(m.Config.SiteURL, m.Config.Mailer.MemberFolder, "/confirm/"+user.ConfirmationToken)
	if err != nil {
		return err
	}
	data := map[string]interface{}{
		"SiteURL":         m.Config.SiteURL,
		"ConfirmationURL": url,
		"Email":           user.Email,
		"Token":           user.ConfirmationToken,
		"Data":            user.UserMetaData,
	}

	return m.TemplateMailer.Mail(
		user.Email,
		withDefault(m.Config.Mailer.Subjects.Confirmation, "Confirm Your Signup"),
		m.Config.Mailer.Templates.Confirmation,
		defaultConfirmationMail,
		data,
	)
}

// EmailChangeMail sends an email change confirmation mail to a user
func (m *TemplateMailer) EmailChangeMail(user *models.User) error {
	url, err := getSiteURL(m.Config.SiteURL, m.Config.Mailer.MemberFolder, "/confirm-email/"+user.EmailChangeToken)
	if err != nil {
		return err
	}
	data := map[string]interface{}{
		"SiteURL":         m.Config.SiteURL,
		"ConfirmationURL": url,
		"Email":           user.Email,
		"NewEmail":        user.EmailChange,
		"Token":           user.EmailChangeToken,
		"Data":            user.UserMetaData,
	}

	return m.TemplateMailer.Mail(
		user.EmailChange,
		withDefault(m.Config.Mailer.Subjects.EmailChange, "Confirm Email Change"),
		m.Config.Mailer.Templates.EmailChange,
		defaultEmailChangeMail,
		data,
	)
}

// RecoveryMail sends a password recovery mail
func (m *TemplateMailer) RecoveryMail(user *models.User) error {
	url, err := getSiteURL(m.Config.SiteURL, m.Config.Mailer.MemberFolder, "/recover/"+user.RecoveryToken)
	if err != nil {
		return err
	}
	data := map[string]interface{}{
		"SiteURL":         m.Config.SiteURL,
		"ConfirmationURL": url,
		"Email":           user.Email,
		"Token":           user.RecoveryToken,
		"Data":            user.UserMetaData,
	}

	return m.TemplateMailer.Mail(
		user.Email,
		withDefault(m.Config.Mailer.Subjects.Recovery, "Reset Your Password"),
		m.Config.Mailer.Templates.Recovery,
		defaultRecoveryMail,
		data,
	)
}

func withDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

// Send can be used to send one-off emails to users
func (m TemplateMailer) Send(user *models.User, subject, body string, data map[string]interface{}) error {
	return m.TemplateMailer.Mail(
		user.Email,
		subject,
		"",
		body,
		data,
	)
}

func (m noopMailer) ValidateEmail(email string) error {
	return nil
}

func (m *noopMailer) InviteMail(user *models.User) error {
	return nil
}

func (m *noopMailer) ConfirmationMail(user *models.User) error {
	return nil
}

func (m noopMailer) RecoveryMail(user *models.User) error {
	return nil
}

func (m *noopMailer) EmailChangeMail(user *models.User) error {
	return nil
}

func (m noopMailer) Send(user *models.User, subject, body string, data map[string]interface{}) error {
	return nil
}

func getSiteURL(siteURL, folder, filename string) (string, error) {
	site, err := url.Parse(siteURL)
	if err != nil {
		return "", err
	}
	path, err := url.Parse(path.Join(folder, filename))
	if err != nil {
		return "", err
	}
	return site.ResolveReference(path).String(), nil
}
