package EVEAuth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/joho/godotenv"
	"github.com/lestrrat-go/jwx/jwk"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var conf = oauth2.Config{
	RedirectURL:  "",
	ClientID:     "",
	ClientSecret: "",
	Scopes: []string{
		"esi-wallet.read_character_wallet.v1",
		"esi-wallet.read_corporation_wallet.v1",
		"esi-wallet.read_corporation_wallets.v1"},
	Endpoint: oauth2.Endpoint{
		AuthURL:  "https://login.eveonline.com/v2/oauth/authorize/",
		TokenURL: "https://login.eveonline.com/v2/oauth/token",
	},
}
const redirectURL = "https://eve-income.web.app/login?token=%s&eve_token=%s"
const imageURL = "https://images.evetech.net/characters/%s/portrait?tenant=tranquility"
const jwkKeyURL = "https://login.eveonline.com/oauth/jwks"

type EVEClaim struct {
	jwt.StandardClaims
	Scp    []string `json:"scp"`
	Jti    string   `json:"jti"`
	Kid    string   `json:"kid"`
	Sub    string   `json:"sub"`
	Azp    string   `json:"azp"`
	Tenant string   `json:"tenant"`
	Tier   string   `json:"tier"`
	Region string   `json:"region"`
	Name   string   `json:"name"`
	Owner  string   `json:"owner"`
	Exp    int      `json:"exp"`
	Iss    string   `json:"iss"`
}

type WriteData struct {
	CharacterID  string
	Code string
}

//Handler is EntryPoint for cloud functions.
func Handler(w http.ResponseWriter, r *http.Request) {
	err := godotenv.Load(".env")
	if err != nil {
		// .env読めなかった場合の処理
	}
	conf.RedirectURL = os.Getenv("RedirectURL")
	conf.ClientID = os.Getenv("ClientID")
	conf.ClientSecret = os.Getenv("ClientSecret")


	//no parameter
	err = r.ParseForm()
	if err != nil {
		redirect(w, r)
		return
	}

	//state check
	stateFromCookie, _ := r.Cookie("__session")
	state := r.Form.Get("state")
	if state != stateFromCookie.Value{
		fmt.Fprint(w, "state error")
		return
	}

	//no has code
	code := r.Form.Get("code")
	if code == "" {
		redirect(w, r)
		return
	}

	//
	token := Exchange(code)
	conf.TokenSource(context.TODO(),token)

	EVEAccesstoken,firebaseToken := initializeFirebase(code)

	redirect := fmt.Sprintf(redirectURL, firebaseToken, EVEAccesstoken)
	http.Redirect(w, r, redirect, http.StatusMovedPermanently)
}

func redirect(w http.ResponseWriter, r *http.Request) {
	state := generateStateOauthCookie(w)
	url := conf.AuthCodeURL(os.Getenv(state))
	http.Redirect(w, r, url, 301)
}

func getJWKKey(token *jwt.Token) (interface{}, error) {

	kid := fmt.Sprintf("%v", token.Header["kid"])

	set, err := jwk.FetchHTTP(jwkKeyURL)
	if err != nil {
		log.Printf("failed to parse JWK: %s", err)
		return nil, err
	}

	keys := set.LookupKeyID(kid)
	var publicKey interface{}
	keys[0].Raw(&publicKey)
	return publicKey, nil
}

func fetchClaim(token string) *EVEClaim {
	claim := &EVEClaim{}
	_, err := jwt.ParseWithClaims(token, claim, getJWKKey)
	if err != nil {
		log.Fatal(err)
	}
	return claim
}

func initializeFirebase(code string) (string,string) {
	token := Exchange(code)
	claim := fetchClaim(token.AccessToken)
	uid := claim.Owner + claim.Sub

	//firebase init
	opt := option.WithCredentialsJSON([]byte(os.Getenv("AdminJsonFile")))
	app, _ := firebase.NewApp(context.Background(), nil, opt)
	client, _ := app.Auth(context.Background())
	firestore, err := app.Firestore(context.Background())

	if err != nil {
		log.Fatal(err)
	}

	//write firestore
	characterID := strings.Split(claim.Sub, ":")[2]
	firestore.Doc("access_token/" + uid).Set(context.Background(), WriteData{characterID, code})

	// user update or create
	var userUpdate = &auth.UserToUpdate{}
	userUpdate.DisplayName(claim.Name)
	userUpdate.PhotoURL(fmt.Sprintf(imageURL, characterID))
	_, err = client.UpdateUser(context.Background(), uid, userUpdate)
	if err != nil {
		// if update fail. create new user.
		var user = &auth.UserToCreate{}
		user.DisplayName(claim.Name)
		user.PhotoURL(fmt.Sprintf(imageURL, characterID))
		user.UID(uid)
		client.CreateUser(context.Background(), user)
	}

	result, err := client.CustomToken(context.Background(), uid)

	if err != nil {
		log.Fatal(err)
	}
	return token.AccessToken,result
}

//トークンの分解。
func Exchange(authCode string) *oauth2.Token {

	token, err := conf.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatal(err)
	}
	return token
}

func generateStateOauthCookie(w http.ResponseWriter) string {
	var expiration = time.Now().Add(365 * 24 * time.Hour)

	b := make([]byte, 16)
	rand.Read(b)
	state := base64.URLEncoding.EncodeToString(b)
	cookie := http.Cookie{Name: "__session", Value: state, Expires: expiration}
	w.Header().Set("Cache-Control","private")
	http.SetCookie(w, &cookie)

	return state
}