
cloud functionでEVEOnlineの認証、管理を行うためのコード。

# やってること

* codeがない場合はリダイレクト
* codeが帰ってきたら
    * stateのチェック
    * accesstokenの有効性チェック
    * ユーザーの新規登録 or 更新
    * firebaseとEVEOnlineのトークンを返却。

# command

```sh
$ go mod tidy
```

# deploy

```sh

$ gcloud functions deploy Callback --runtime go113 --trigger-http --allow-unauthenticated --env-vars-file .env.yaml --region asia-northeast1

```

## .env.yaml example
```yaml
RedirectURL: 'http://localhost:8080'
ClientID: 'hoge'
ClientSecret: 'fuga'
AdminJsonFile: ''
```

https://cloud.google.com/functions/docs/deploying?hl=ja


## firesotre.rules example
```
rules_version = '2';
service cloud.firestore {
  match /databases/{database}/documents {
    match /access_token/{token} {
      allow read, update, delete: if request.auth != null && request.auth.uid == token;
      allow create: if request.auth != null;
    }
  }
}

```

## EVEOnline 公式ドキュメント周り
* https://github.com/esi/esi-docs
* https://esi.evetech.net/ui/
* https://developers.eveonline.com/applications

# reference
https://github.com/douglasmakey/oauth2-example