# バックエンドAPIドキュメント

## 共通仕様
- Base URL: `AWS_API_GATEWAY_URL`(フロントエンドプロジェクトの.env.localを参照)
- 認証: API (API Token)
- 共通ヘッダー:
  - `Content-Type: application/json`
  - `x-api-key: AWS_API_GATEWAY_KEY_PROD`(フロントエンドプロジェクトの.env.localを参照)

---

## エンドポイント一覧
- `/drafts` 下書きブログデータベースの操作
- `/drafts/{id}` 下書きブログデータベース内にある特定のidを持つオブジェクトに対して操作する
- `posts` 本番用ブログデータベースの操作
- （必要に応じて追加）

---

## エンドポイント詳細

### POST /drafts
- 概要: 下書きブログデータベースにアイテムを挿入する
- 認証: API Key

#### Request
```json
[
  {
    "title": "Example Post",
    "date": "2025-08-26",
    "content": "This is the content of my first blog post.",
    "tags": [
      "Go",
      "AWS",
      "Lambda"
    ],
    "isPublished": false
  }
]
```

#### Response 200
```json
[
  { "id": "21828f55-1bb6-4a2f-abcc-79e3453f0d8f" },
]
```

#### Error
- 400 Bad Request
```json
[
  { "error": "Invalid request body" },
]
```

- 500 Internal Server Error
```json
[
  { "error": "Failed to marshal item: {err}" },
]
```

```json
[
  { "error": "Failed to put item to DynamoDB: {err}" },
]
```

### DELETE /drafts/{id}
- 概要: 下書きブログデータベースに{id}を持ったアイテムを削除する
- 認証: API Key

#### Request
```json
[]
```

#### Response 200
```json
[
  {
    "id": "id",
    "title": "Example Post",
    "date": "2025-08-26",
    "content": "This is the content of my first blog post.",
    "tags": [
      "Go",
      "AWS",
      "Lambda"
    ],
    "isPublished": false,
    "ttl": 10000000
  }
]
```

#### Error
- 404 Not Found
```json
[
  { "error": "指定された主キーを持つアイテムは見つかりませんでした" },
]
```

- 500 Internal Server Error
```json
[
  { "error": "Failed to delete draft: {err}" },
]
```

### GET /drafts/{id}
- 概要: 下書きブログデータベースに{id}を持ったアイテムを取得する
- 認証: API Key

#### Request
```json
[]
```

#### Response 200
```json
[
  { "messeage": "Draft with ID {id} deleted successfully" },
]
```

#### Error
- 400 Bad Request
```json
[
  { "error": "Missing draft ID" },
]
```

- 500 Internal Server Error
```json
[
  { "error": "アイテムの取得に失敗しました" },
]
```
```json
[
  { "error": "アイテムのパースに失敗しました" },
]
```
```json
[
  { "error": "レスポンスボディの作成に失敗しました" },
]
```


