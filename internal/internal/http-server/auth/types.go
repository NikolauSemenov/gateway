package auth

type RegisterRequest struct {
	Login    string `json:"login"`
	UserName string `json:"user_name"`
	LastName string `json:"last_name"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	UserName    string `json:"user_name"`
	LastName    string `json:"last_name"`
}

type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type SendEmailCodeRequest struct {
	Email string `json:"email"`
}

type VerifyEmailCodeRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type VerifyEmailCodeResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	UserName    string `json:"user_name"`
	LastName    string `json:"last_name"`
}

type VerifyRequest struct {
	Authorization string `json:"Authorization"`
}
