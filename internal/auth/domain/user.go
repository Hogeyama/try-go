package authdomain

import "time"

type User struct {
	ID           int32     `json:"id"`
	Username     string    `json:"username"`
	Role         Role      `json:"roles"`
	PasswordHash string    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Role int

const (
	RoleSystemAdmin Role = iota
	RoleAdmin
	RoleWriter
	RoleReader
)

func (r Role) String() string {
	return roleToString[r]
}

func RoleFromString(s string) Role {
	return stringToRole[s]
}

var roleToString = map[Role]string{
	RoleSystemAdmin: "sysadmin",
	RoleAdmin:       "admin",
	RoleWriter:      "writer",
	RoleReader:      "reader",
}

var stringToRole = map[string]Role{
	"sysadmin": RoleSystemAdmin,
	"admin":    RoleAdmin,
	"writer":   RoleWriter,
	"reader":   RoleReader,
}
