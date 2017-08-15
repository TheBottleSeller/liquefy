package models

type User struct {
	ID                    uint   `gorm:primary_key json:"id"`
	ApiKey                string `json:"apiKey"`
	Username              string `json:"username"`
	Password              string `json:"password"`
	Firstname             string `json:"firstname"`
	Lastname              string `json:"lastname"`
	Email                 string `json:"email"`
	PublicID              string `json:"publicId"`
	AwsAccountID          uint   `json:"awsAccountId"`
	GithubOauthToken      string `json:"githubOauthToken"`
	BitbucketOauthToken   string `json:"bitbucketOauthToken"`
	BitbucketRefreshToken string `json:"bitbucketRefreshToken"`
}
