package ldap

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	goldap "github.com/go-ldap/ldap/v3"
)

type Client struct {
	URL           string
	BaseDN        string
	AdminDN       string
	AdminPassword string
}

type User struct {
	DN          string `json:"dn"`
	UID         string `json:"uid"`
	CN          string `json:"cn"`
	Mail        string `json:"mail,omitempty"`
	UIDNumber   string `json:"uidNumber,omitempty"`
	GIDNumber   string `json:"gidNumber,omitempty"`
	HomeDir     string `json:"homeDirectory,omitempty"`
	LoginShell  string `json:"loginShell,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

type Group struct {
	DN          string   `json:"dn"`
	CN          string   `json:"cn"`
	GIDNumber   string   `json:"gidNumber,omitempty"`
	Description string   `json:"description,omitempty"`
	MemberUIDs  []string `json:"memberUids,omitempty"`
}

type OrganizationalUnit struct {
	DN          string `json:"dn"`
	OU          string `json:"ou"`
	Description string `json:"description,omitempty"`
}

type CreateUserRequest struct {
	Username, DisplayName, Email, Password, UIDNumber, GIDNumber, HomeDirectory string
}

type CreateGroupRequest struct {
	Name, GIDNumber, Description string
}

func New(url, baseDN, adminDN, adminPassword string) *Client {
	return &Client{URL: url, BaseDN: baseDN, AdminDN: adminDN, AdminPassword: adminPassword}
}

func (c *Client) Ping() error {
	conn, err := c.bind()
	if err != nil {
		return err
	}
	defer conn.Close()
	return nil
}

// Authenticate verifies a user's password with a direct LDAP bind.
func (c *Client) Authenticate(username, password string) (User, error) {
	if strings.TrimSpace(password) == "" {
		return User{}, fmt.Errorf("password is required")
	}
	adminConn, err := c.bind()
	if err != nil {
		return User{}, err
	}
	dn, err := c.userDN(adminConn, username)
	if err != nil {
		adminConn.Close()
		return User{}, err
	}
	adminConn.Close()

	conn, err := goldap.DialURL(c.URL)
	if err != nil {
		return User{}, err
	}
	defer conn.Close()
	if err := conn.Bind(dn, password); err != nil {
		return User{}, fmt.Errorf("invalid credentials")
	}

	req := goldap.NewSearchRequest(
		dn,
		goldap.ScopeBaseObject,
		goldap.NeverDerefAliases,
		1,
		5,
		false,
		"(objectClass=*)",
		[]string{"dn", "uid", "cn", "mail", "uidNumber", "gidNumber", "homeDirectory", "loginShell", "displayName"},
		nil,
	)
	result, err := conn.Search(req)
	if err != nil || len(result.Entries) == 0 {
		if err != nil {
			return User{}, err
		}
		return User{}, fmt.Errorf("ldap user %s not found", username)
	}
	entry := result.Entries[0]
	return User{
		DN:          entry.DN,
		UID:         entry.GetAttributeValue("uid"),
		CN:          entry.GetAttributeValue("cn"),
		Mail:        entry.GetAttributeValue("mail"),
		UIDNumber:   entry.GetAttributeValue("uidNumber"),
		GIDNumber:   entry.GetAttributeValue("gidNumber"),
		HomeDir:     entry.GetAttributeValue("homeDirectory"),
		LoginShell:  entry.GetAttributeValue("loginShell"),
		DisplayName: entry.GetAttributeValue("displayName"),
	}, nil
}

func (c *Client) EnsureBaseOUs() ([]string, error) {
	conn, err := c.bind()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	ous := []string{"users", "groups", "teams", "units"}
	created := make([]string, 0, len(ous))
	for _, ou := range ous {
		dn := fmt.Sprintf("ou=%s,%s", ou, c.BaseDN)
		exists, err := c.exists(conn, dn)
		if err != nil {
			return created, err
		}
		if exists {
			continue
		}
		req := goldap.NewAddRequest(dn, nil)
		req.Attribute("objectClass", []string{"top", "organizationalUnit"})
		req.Attribute("ou", []string{ou})
		if err := conn.Add(req); err != nil {
			return created, err
		}
		created = append(created, dn)
	}
	return created, nil
}

func (c *Client) ListUsers() ([]User, error) {
	conn, err := c.bind()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	base := "ou=users," + c.BaseDN
	req := goldap.NewSearchRequest(
		base,
		goldap.ScopeWholeSubtree,
		goldap.NeverDerefAliases,
		100,
		int(5*time.Second/time.Second),
		false,
		"(|(objectClass=posixAccount)(objectClass=inetOrgPerson))",
		[]string{"dn", "uid", "cn", "mail", "uidNumber", "gidNumber", "homeDirectory", "loginShell", "displayName"},
		nil,
	)
	result, err := conn.Search(req)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such object") {
			return []User{}, nil
		}
		return nil, err
	}
	users := make([]User, 0, len(result.Entries))
	for _, entry := range result.Entries {
		users = append(users, User{
			DN:          entry.DN,
			UID:         entry.GetAttributeValue("uid"),
			CN:          entry.GetAttributeValue("cn"),
			Mail:        entry.GetAttributeValue("mail"),
			UIDNumber:   entry.GetAttributeValue("uidNumber"),
			GIDNumber:   entry.GetAttributeValue("gidNumber"),
			HomeDir:     entry.GetAttributeValue("homeDirectory"),
			LoginShell:  entry.GetAttributeValue("loginShell"),
			DisplayName: entry.GetAttributeValue("displayName"),
		})
	}
	return users, nil
}

func (c *Client) SetUserDisabled(username string, disabled bool) error {
	conn, err := c.bind()
	if err != nil {
		return err
	}
	defer conn.Close()
	dn, err := c.userDN(conn, username)
	if err != nil {
		return err
	}
	shell := "/bin/bash"
	if disabled {
		shell = "/sbin/nologin"
	}
	req := goldap.NewModifyRequest(dn, nil)
	req.Replace("loginShell", []string{shell})
	return conn.Modify(req)
}

func (c *Client) SetUserPassword(username, password string) error {
	conn, err := c.bind()
	if err != nil {
		return err
	}
	defer conn.Close()
	dn, err := c.userDN(conn, username)
	if err != nil {
		return err
	}
	hashed, err := sshaPassword(password)
	if err != nil {
		return err
	}
	req := goldap.NewModifyRequest(dn, nil)
	req.Replace("userPassword", []string{hashed})
	return conn.Modify(req)
}

func (c *Client) DeleteUser(username string) error {
	conn, err := c.bind()
	if err != nil {
		return err
	}
	defer conn.Close()
	dn, err := c.userDN(conn, username)
	if err != nil {
		return err
	}
	return conn.Del(goldap.NewDelRequest(dn, nil))
}

func (c *Client) CreateUser(input CreateUserRequest) error {
	conn, err := c.bind()
	if err != nil {
		return err
	}
	defer conn.Close()
	dn := fmt.Sprintf("uid=%s,ou=users,%s", goldap.EscapeDN(strings.TrimSpace(input.Username)), c.BaseDN)
	if exists, err := c.exists(conn, dn); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("ldap user %s already exists", input.Username)
	}
	hashed, err := sshaPassword(input.Password)
	if err != nil {
		return err
	}
	req := goldap.NewAddRequest(dn, nil)
	req.Attribute("objectClass", []string{"top", "person", "organizationalPerson", "inetOrgPerson", "posixAccount", "shadowAccount"})
	req.Attribute("uid", []string{input.Username})
	req.Attribute("cn", []string{input.DisplayName})
	req.Attribute("sn", []string{input.DisplayName})
	req.Attribute("displayName", []string{input.DisplayName})
	req.Attribute("mail", []string{input.Email})
	req.Attribute("uidNumber", []string{input.UIDNumber})
	req.Attribute("gidNumber", []string{input.GIDNumber})
	req.Attribute("homeDirectory", []string{input.HomeDirectory})
	req.Attribute("loginShell", []string{"/bin/bash"})
	req.Attribute("userPassword", []string{hashed})
	return conn.Add(req)
}

func (c *Client) UpdateUser(username, displayName, email string) error {
	conn, err := c.bind()
	if err != nil {
		return err
	}
	defer conn.Close()
	dn, err := c.userDN(conn, username)
	if err != nil {
		return err
	}
	req := goldap.NewModifyRequest(dn, nil)
	req.Replace("cn", []string{displayName})
	req.Replace("sn", []string{displayName})
	req.Replace("displayName", []string{displayName})
	req.Replace("mail", []string{email})
	return conn.Modify(req)
}

func (c *Client) CreateGroup(input CreateGroupRequest) error {
	conn, err := c.bind()
	if err != nil {
		return err
	}
	defer conn.Close()
	dn := fmt.Sprintf("cn=%s,ou=groups,%s", goldap.EscapeDN(strings.TrimSpace(input.Name)), c.BaseDN)
	if exists, err := c.exists(conn, dn); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("ldap group %s already exists", input.Name)
	}
	req := goldap.NewAddRequest(dn, nil)
	req.Attribute("objectClass", []string{"top", "posixGroup"})
	req.Attribute("cn", []string{input.Name})
	req.Attribute("gidNumber", []string{input.GIDNumber})
	if input.Description != "" {
		req.Attribute("description", []string{input.Description})
	}
	return conn.Add(req)
}

func (c *Client) AddGroupMember(groupName, username string) error {
	conn, err := c.bind()
	if err != nil {
		return err
	}
	defer conn.Close()
	req := goldap.NewSearchRequest(c.BaseDN, goldap.ScopeWholeSubtree, goldap.NeverDerefAliases, 1, 5, false, fmt.Sprintf("(cn=%s)", goldap.EscapeFilter(groupName)), []string{"dn", "memberUid"}, nil)
	result, err := conn.Search(req)
	if err != nil {
		return err
	}
	if len(result.Entries) == 0 {
		return nil
	}
	for _, value := range result.Entries[0].GetAttributeValues("memberUid") {
		if value == username {
			return nil
		}
	}
	modify := goldap.NewModifyRequest(result.Entries[0].DN, nil)
	modify.Add("memberUid", []string{username})
	return conn.Modify(modify)
}

func (c *Client) DeleteGroup(groupName string) error {
	conn, err := c.bind()
	if err != nil {
		return err
	}
	defer conn.Close()
	req := goldap.NewSearchRequest(c.BaseDN, goldap.ScopeWholeSubtree, goldap.NeverDerefAliases, 1, 5, false, fmt.Sprintf("(&(objectClass=posixGroup)(cn=%s))", goldap.EscapeFilter(groupName)), []string{"dn"}, nil)
	result, err := conn.Search(req)
	if err != nil {
		return err
	}
	if len(result.Entries) == 0 {
		return nil
	}
	return conn.Del(goldap.NewDelRequest(result.Entries[0].DN, nil))
}

func (c *Client) ListGroups() ([]Group, error) {
	groups := []Group{}
	for _, ou := range []string{"groups", "teams"} {
		items, err := c.listGroupsInOU(ou)
		if err != nil {
			return nil, err
		}
		groups = append(groups, items...)
	}
	return groups, nil
}

func (c *Client) listGroupsInOU(ou string) ([]Group, error) {
	conn, err := c.bind()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	base := fmt.Sprintf("ou=%s,%s", ou, c.BaseDN)
	req := goldap.NewSearchRequest(
		base,
		goldap.ScopeWholeSubtree,
		goldap.NeverDerefAliases,
		200,
		5,
		false,
		"(|(objectClass=posixGroup)(objectClass=groupOfNames)(objectClass=groupOfUniqueNames))",
		[]string{"dn", "cn", "gidNumber", "description", "memberUid"},
		nil,
	)
	result, err := conn.Search(req)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such object") {
			return []Group{}, nil
		}
		return nil, err
	}
	groups := make([]Group, 0, len(result.Entries))
	for _, entry := range result.Entries {
		groups = append(groups, Group{
			DN:          entry.DN,
			CN:          entry.GetAttributeValue("cn"),
			GIDNumber:   entry.GetAttributeValue("gidNumber"),
			Description: entry.GetAttributeValue("description"),
			MemberUIDs:  entry.GetAttributeValues("memberUid"),
		})
	}
	return groups, nil
}

func (c *Client) ListOrganizationalUnits() ([]OrganizationalUnit, error) {
	conn, err := c.bind()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	base := "ou=units," + c.BaseDN
	req := goldap.NewSearchRequest(
		base,
		goldap.ScopeWholeSubtree,
		goldap.NeverDerefAliases,
		200,
		5,
		false,
		"(objectClass=organizationalUnit)",
		[]string{"dn", "ou", "description"},
		nil,
	)
	result, err := conn.Search(req)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such object") {
			return []OrganizationalUnit{}, nil
		}
		return nil, err
	}
	units := make([]OrganizationalUnit, 0, len(result.Entries))
	for _, entry := range result.Entries {
		name := entry.GetAttributeValue("ou")
		if strings.EqualFold(name, "units") {
			continue
		}
		units = append(units, OrganizationalUnit{
			DN:          entry.DN,
			OU:          name,
			Description: entry.GetAttributeValue("description"),
		})
	}
	return units, nil
}

func (c *Client) bind() (*goldap.Conn, error) {
	if c.AdminPassword == "" {
		return nil, fmt.Errorf("LDAP_ADMIN_PASSWORD is not configured")
	}
	conn, err := goldap.DialURL(c.URL)
	if err != nil {
		return nil, err
	}
	if err := conn.Bind(c.AdminDN, c.AdminPassword); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func (c *Client) userDN(conn *goldap.Conn, username string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return "", fmt.Errorf("username is required")
	}
	req := goldap.NewSearchRequest(
		"ou=users,"+c.BaseDN,
		goldap.ScopeWholeSubtree,
		goldap.NeverDerefAliases,
		1,
		5,
		false,
		fmt.Sprintf("(uid=%s)", goldap.EscapeFilter(username)),
		[]string{"dn"},
		nil,
	)
	result, err := conn.Search(req)
	if err != nil {
		return "", err
	}
	if len(result.Entries) == 0 {
		return "", fmt.Errorf("ldap user %s not found", username)
	}
	return result.Entries[0].DN, nil
}

func sshaPassword(password string) (string, error) {
	salt := make([]byte, 8)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	h := sha1.New()
	h.Write([]byte(password))
	h.Write(salt)
	sum := h.Sum(nil)
	return "{SSHA}" + base64.StdEncoding.EncodeToString(append(sum, salt...)), nil
}

func (c *Client) exists(conn *goldap.Conn, dn string) (bool, error) {
	req := goldap.NewSearchRequest(
		dn,
		goldap.ScopeBaseObject,
		goldap.NeverDerefAliases,
		1,
		5,
		false,
		"(objectClass=*)",
		[]string{"dn"},
		nil,
	)
	_, err := conn.Search(req)
	if err == nil {
		return true, nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "no such object") {
		return false, nil
	}
	return false, err
}
