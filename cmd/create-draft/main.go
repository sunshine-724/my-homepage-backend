package main

import (
	"context"
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
var region = os.Getenv("AWS_REGION")

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
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: "Unsupported media type"}, nil
	}

	var reqBody RequestBody

	var item DraftItem // dynamoDBに保存するデータ
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
		boundary := params["boundary"]

		mr := multipart.NewReader(strings.NewReader(request.Body), boundary) // multipart/form-dataのリクエストボディをパース
		for {
			part, err := mr.NextPart()

			// エラー処理
			if err == io.EOF {
				break
			}
			if err != nil {
				return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Failed to read part: %v", err)}, nil
			}

			formName := part.FormName() // Content-Dispositionヘッダーからフォーム名を取得

			if part.FileName() != "" {
				// ファイルがアップロードされている場合
				/* S3処理 */
				s3ObjectKey := fmt.Sprintf("%s/%s", draftID, part.FileName()) // オブジェクトキーにIDを含める

				fileBytes, err := io.ReadAll(part) // アップロードされたファイルの内容を読み取る
				if err != nil {
					return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Failed to read file: %v", err)}, nil
				}

				// S3にファイルをアップロード
				_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
					Bucket: aws.String(bucketName),
					Key:    aws.String(s3ObjectKey),
					Body:   strings.NewReader(string(fileBytes)),
				})

				if err != nil {
					return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Failed to upload file to S3: %v", err)}, nil
				}

				attachmentFilePaths = append(attachmentFilePaths, s3ObjectKey) // オブジェクトキーを保存
			} else if formName == "metadata" {
				// テキストデータだった場合
				bodyBytes, err := io.ReadAll(part)
				if err != nil {
					return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Failed to read form data: %v", err)}, nil
				}
				if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
					return events.APIGatewayProxyResponse{StatusCode: 400, Body: fmt.Sprintf("Invalid JSON data: %v", err)}, nil
				}
			}
		}
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
