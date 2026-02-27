sequenceDiagram
  autonumber
  participant B as Browser (Admin UI)
  participant N as Next.js API Routes
  participant G as AWS API Gateway
  participant L as AWS Lambda
  participant D as DynamoDB (Drafts/Posts)

  %% --- 1. 下書き作成フェーズ ---
  Note over B, D: 1. 下書きの作成 (/admin/blog/makeBlogPage)
  
  B->>N: POST /api/blog/postDraftTable
  note right of B: Body: JSON or FormData
  N->>G: POST /drafts
  note right of N: Header: x-api-key<br/>Body: DraftCreateRequest
  G->>L: invoke (POST /drafts)
  L->>D: 下書きを保存 (PutItem)
  D-->>L: 保存成功
  L-->>G: 200 { id }
  G-->>N: 200 { id }
  N-->>B: 200 { id }
  B->>B: router.push(/admin/blog/preview?encodedID=id)

  %% --- 2. プレビューフェーズ ---
  Note over B, D: 2. プレビュー表示 (/admin/blog/preview)
  
  B->>N: POST /api/blog/getDraftTableItem
  note right of B: Body: { id }
  N->>G: GET /drafts/{id}
  note right of N: Header: x-api-key
  G->>L: invoke (GET /drafts/{id})
  L->>D: 下書きを取得 (GetItem)
  D-->>L: Draft レコード
  L-->>G: 200 Draft
  G-->>N: 200 Draft
  N-->>B: 200 BlogDetail
  B->>B: Markdown プレビューをレンダリング

  %% --- 3. 公開フェーズ ---
  Note over B, D: 3. 記事の公開 (「投稿する」クリック)
  
  B->>N: POST /api/blog/publishPostTable
  note right of B: Body: { id, isPublished: true }
  N->>G: POST /posts
  note right of N: Header: x-api-key<br/>Body: PostPublishRequest
  G->>L: invoke (POST /posts)
  L->>D: Draftsから取得 & Postsへ保存 / isPublished更新
  D-->>L: 処理完了
  L-->>G: 200 { id, message }
  G-->>N: 200 { id, message }
  N-->>B: 200 { id, message }
  B->>B: alert("公開成功") + router.push(/)