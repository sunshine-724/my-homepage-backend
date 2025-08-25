# バックエンドAPIドキュメント

## 共通仕様
- Base URL: `AWS_API_GATEWAY_URL`(フロントエンドプロジェクトの.env.localを参照)
- 認証: API (API Token)
- 共通ヘッダー:
  - `Content-Type: application/json`
  - `x-api-key: AWS_API_GATEWAY_KEY_PROD`(フロントエンドプロジェクトの.env.localを参照)

---

## エンドポイント一覧
- `/posts` ブログデータベースの操作
---

## エンドポイント詳細

### POST /posts
- 概要: 下書き用データベースからブログデータベースにアイテムを挿入する
- 認証: API Key

#### Request
```json
[
  {
    "id": "id",
    "isPublished": true
  }
]
```

#### Response 200
```json
[
  { 
    "id": "id",
    "message": "Blog post published successfully!"
  }
]
```

#### Error
- 400 Bad Request
```json
[
  { "error": "Invalid request body" },
]
```

- 404 Not Found
```json
[
  { "error": "Draft with ID {id} not found" },
]
```

- 500 Internal Server Error
```json
[
  { "error": "Failed to unmarshal draft: {err}" },
]
```
```json
[
  { "error": "Failed to marshal post: {err}" },
]
```
```json
[
  { "error": "Failed to put post to DynamoDB: {err}" },
]
```