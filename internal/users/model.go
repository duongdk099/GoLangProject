// Package users owns accounts, skills, and the welcome-credit journal entry.
package users

type User struct {
	ID            int     `json:"id"`
	Pseudo        string  `json:"pseudo"`
	Bio           string  `json:"bio,omitempty"`
	Ville         string  `json:"ville,omitempty"`
	Skills        []Skill `json:"skills,omitempty"`
	CreditBalance int     `json:"credit_balance"`
	CreatedAt     string  `json:"created_at"`
}

type Skill struct {
	Nom    string `json:"nom"`
	Niveau string `json:"niveau"`
}

type CreateUserRequest struct {
	Pseudo string `json:"pseudo"`
	Bio    string `json:"bio"`
	Ville  string `json:"ville"`
}

type UpdateUserRequest struct {
	Pseudo string `json:"pseudo"`
	Bio    string `json:"bio"`
	Ville  string `json:"ville"`
}

type CreateUserParams struct {
	Pseudo string
	Bio    string
	Ville  string
}

type UpdateUserParams struct {
	Pseudo string
	Bio    string
	Ville  string
}
