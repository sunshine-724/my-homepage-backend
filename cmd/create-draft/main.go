package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// TODO: ロギングをfmtからzerologのような構造化ロギングライブラリに移行する (LOG_LEVEL環境変数で制御)


// RequestBody: フロントエンドから送られてくるリクエストボディ
type RequestBody struct {
	Title       string   `json:"title"`
	Date        string   `json:"date"`
	Content     string   `json:"content"`
	Tags        []string `json:"tags"`
	IsPublished bool     `json:"isPublished"`
}

// DraftItem: DynamoDBに保存するデータ構造
type DraftItem struct {
	ID                 string   `dynamodbav:"id"` // DynamoDB に送るキー名を明示
	Title              string   `dynamodbav:"title"`
	Date               string   `dynamodbav:"date"`
	Content            string   `dynamodbav:"content"`
	Tags               []string `dynamodbav:"tags"`
	AttachmentFilePath []string `dynamodbav:"attachmentFilePath"` // S3に保存したファイルのパス
	IsPublished        bool     `dynamodbav:"isPublished"`
	TTL                int64    `dynamodbav:"ttl"`
}

var dbClient *dynamodb.Client
var tableName = os.Getenv("TABLE_NAME")

var s3Client *s3.Client
var bucketName = os.Getenv("BUCKET_NAME")
var region = os.Getenv("AWS_REGION") // AWS側で環境変数を取得してくれる

func init() {
	// v2ではconfig.LoadDefaultConfigを使って設定をロード
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		// エラー処理
	}
	// DynamoDBクライアントをv2で作成
	dbClient = dynamodb.NewFromConfig(cfg)
	s3Client = s3.NewFromConfig(cfg)
}

func Handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	/* 入力処理 */
	contentType := request.Headers["content-type"]
	if contentType == "" {
		contentType = request.Headers["Content-Type"] // 大文字小文字の違いを吸収
	}

	mediaType, params, err := mime.ParseMediaType(contentType) // params: セミコロン以降のパラメータ(ex.map[boundary]----abc)
	if err != nil || !(strings.HasPrefix(mediaType, "multipart/") || strings.HasPrefix(mediaType, "application/")) {
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: "Unsupported media type"}, nil
	}

	var reqBody RequestBody

	var item DraftItem               // dynamoDBに保存するデータ
	var attachmentFilePaths []string // S3に保存したファイルのパス

	var draftID string // dynamoDBの主キー
	var ttl int64

	draftID = uuid.New().String()
	fmt.Println("Generated draftID:", draftID)
	ttl = time.Now().Add(7 * 24 * time.Hour).Unix()

	if mediaType == "application/json" {
		/* 送られてきたのがJSONのテキスト形式だった場合 */
		if err := json.Unmarshal([]byte(request.Body), &reqBody); err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 400, Body: fmt.Sprintf("Failed to unmarshal request body: %v", err)}, nil
		}
	} else if strings.HasPrefix(mediaType, "multipart/form-data") {
		/* 送られてきたのが複数ファイルを含むフォームデータだった場合 */

		var bodyReader io.Reader
		if request.IsBase64Encoded {
			fmt.Println("Decoding Base64 body...")
			decodedBody, err := base64.StdEncoding.DecodeString(request.Body)
			if err != nil {
				fmt.Println("Base64 Decode Error:", err)
				return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to decode base64 body"}, nil
			}
			// デコード後のボディの先頭200文字を表示
			if len(decodedBody) > 200 {
				fmt.Printf("Decoded Body (first 200 chars): %s\n", string(decodedBody[:200]))
			} else {
				fmt.Printf("Decoded Body: %s\n", string(decodedBody))
			}
			bodyReader = bytes.NewReader(decodedBody)
		} else {
			fmt.Println("Body is not Base64 encoded. Using raw string.")
			bodyReader = strings.NewReader(request.Body)
		}

		boundary := params["boundary"]
		fmt.Printf("Extracted Boundary: %s\n", boundary)

		if request.IsBase64Encoded {
			// Base64エンコードされている場合、デコードする
			decodedBody, err := base64.StdEncoding.DecodeString(request.Body)
			if err != nil {
				return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to decode base64 body"}, nil
			}
			bodyReader = bytes.NewReader(decodedBody)
		} else {
			// エンコードされていない場合（テスト時など）
			bodyReader = strings.NewReader(request.Body)
		}

		mr := multipart.NewReader(bodyReader, boundary) // 修正: デコード後のReaderを使用

		// ▼▼▼ デバッグログ ▼▼▼
		fmt.Println("--- Starting multipart form processing loop ---")
		loopCounter := 0 // ループ回数を数えるためのカウンター

		for {
			fmt.Printf("\n--- Loop iteration %d ---\n", loopCounter)

			part, err := mr.NextPart()

			// エラー処理
			if err == io.EOF {
				fmt.Println("Reached EOF, breaking loop.")
				break
			}
			if err != nil {
				return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Failed to read part: %v", err)}, nil
			}

			if part.FileName() != "" {
				fmt.Printf("Detected file part. FileName: %s\n", part.FileName())

				// ファイルがアップロードされている場合
				/* S3処理 */
				s3ObjectKey := fmt.Sprintf("%s/%s", draftID, part.FileName())

				fileBytes, err := io.ReadAll(part)
				if err != nil {
					return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Failed to read file: %v", err)}, nil
				}

				// S3にファイルをアップロード
				_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
					Bucket: aws.String(bucketName),
					Key:    aws.String(s3ObjectKey),
					Body:   bytes.NewReader(fileBytes),
				})

				if err != nil {
					return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Failed to upload file to S3: %v", err)}, nil
				}

				attachmentFilePaths = append(attachmentFilePaths, s3ObjectKey) // オブジェクトキーを保存
			} else {
				// フォームフィールドの値を読み取る
				bodyBytes, err := io.ReadAll(part)
				if err != nil {
					return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Failed to read form field: %v", err)}, nil
				}
				fieldValue := string(bodyBytes)

				fmt.Printf("Detected text field. FormName: '%s', Value: '%s'\n", part.FormName(), fieldValue)

				// フィールド名に応じて、reqBody構造体に値をセット
				switch part.FormName() {
				case "title":
					reqBody.Title = fieldValue
				case "content":
					reqBody.Content = fieldValue
				case "date":
					reqBody.Date = fieldValue
				case "tags":
					if err := json.Unmarshal(bodyBytes, &reqBody.Tags); err != nil {
						return events.APIGatewayProxyResponse{StatusCode: 400, Body: fmt.Sprintf("Invalid JSON in tags: %v", err)}, nil
					}
				case "isPublished":
					reqBody.IsPublished = (fieldValue == "true")
				}
			}
			loopCounter++
		}

		fmt.Println("--- Finished multipart form processing loop ---")
	}

	/* DB処理 */
	item = DraftItem{
		ID:                 draftID,
		Title:              reqBody.Title,
		Date:               reqBody.Date,
		Content:            reqBody.Content,
		Tags:               reqBody.Tags,
		AttachmentFilePath: attachmentFilePaths,
		IsPublished:        false,
		TTL:                ttl,
	}
	av, err := attributevalue.MarshalMap(item) // Goの構造体の形からDynamoDBの形に変換

	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Failed to marshal item: %v", err)}, nil
	}

	// input変数に値を入れる直前のTableNameの値をログ出力
	fmt.Println("TableName passed to PutItemInput:", tableName)

	// v2のPutItemInputを作成し、PutItemを実行
	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}

	fmt.Println("About to save item with ID:", item.ID)

	_, err = dbClient.PutItem(ctx, input)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Failed to put item to DynamoDB: %v", err)}, nil
	}

	fmt.Println("Saved draftID to DynamoDB:", draftID)

	responseBody, _ := json.Marshal(map[string]string{"id": draftID}) // Goの構造体からJSONの形に変換
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(responseBody),
	}, nil
}

func main() {
	lambda.Start(Handler)
}
