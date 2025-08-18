package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// RequestBody: フロントエンドから送られてくるリクエストボディ
// 投稿時には、下書きのIDと公開フラグを受け取る
type RequestBody struct {
	ID          string `json:"id"`
	IsPublished bool   `json:"isPublished"`
}

// DraftItem: DynamoDBの下書きテーブルと公開用テーブルで共有するデータ構造
type DraftItem struct {
	ID          string    `dynamodbav:"id"`
	Title       string    `dynamodbav:"title"`
	Date			string    `dynamodbav:"date"`
	Content     string    `dynamodbav:"content"`
	Tags        []string  `dynamodbav:"tags"`
	IsPublished bool      `dynamodbav:"isPublished"`
	TTL         int64     `dynamodbav:"ttl"` // 下書きテーブルでのみ有効
}

var dbClient *dynamodb.Client
var draftsTableName = os.Getenv("DRAFTS_TABLE_NAME") // 下書きテーブル名
var postsTableName = os.Getenv("POSTS_TABLE_NAME")   // 投稿テーブル名

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		// ここではエラーをログに出力するだけにして、Lambdaのコールドスタートを妨げないようにする
		fmt.Fprintf(os.Stderr, "Error loading AWS config: %v\n", err)
		// 本番環境では、アプリケーションの起動に失敗した場合の適切なハンドリングを検討
	}
	dbClient = dynamodb.NewFromConfig(cfg)
}

func Handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	fmt.Println("Received request for post handler.")

	var reqBody RequestBody
	err := json.Unmarshal([]byte(request.Body), &reqBody)
	if err != nil {
		fmt.Printf("Error unmarshalling request body: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: "Invalid request body"}, nil
	}

	// 1. blog_drafts テーブルから下書きデータを取得
	getItemInput := &dynamodb.GetItemInput{
		TableName: aws.String(draftsTableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: reqBody.ID},
		},
	}
	fmt.Printf("Getting item from drafts table: %s\n", reqBody.ID)
	result, err := dbClient.GetItem(ctx, getItemInput)
	if err != nil {
		fmt.Printf("Error getting item from DynamoDB: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Failed to get draft: %v", err)}, nil
	}

	if result.Item == nil {
		fmt.Printf("Draft not found with ID: %s\n", reqBody.ID)
		return events.APIGatewayProxyResponse{StatusCode: 404, Body: fmt.Sprintf("Draft with ID %s not found", reqBody.ID)}, nil
	}

	var draftItem DraftItem
	err = attributevalue.UnmarshalMap(result.Item, &draftItem)
	if err != nil {
		fmt.Printf("Error unmarshalling item: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Failed to unmarshal draft: %v", err)}, nil
	}

	// 2. 公開フラグを更新
	draftItem.IsPublished = reqBody.IsPublished
	// 投稿時はTTLをなくすか、別の期間を設定する（blog_postsテーブルにはTTLは通常不要）
	draftItem.TTL = 0 // blog_postsテーブルにTTLは設定しないので0にするか、フィールド自体を削除

	// 3. blog_posts テーブルにデータを保存
	av, err := attributevalue.MarshalMap(draftItem)
	if err != nil {
		fmt.Printf("Error marshalling post item: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Failed to marshal post: %v", err)}, nil
	}

	putItemInput := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(postsTableName),
	}
	fmt.Printf("Putting item to posts table: %s\n", draftItem.ID)
	_, err = dbClient.PutItem(ctx, putItemInput)
	if err != nil {
		fmt.Printf("Error putting item to posts table: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Failed to put post to DynamoDB: %v", err)}, nil
	}

	// 4. blog_drafts テーブルから下書きを削除
	deleteItemInput := &dynamodb.DeleteItemInput{
		TableName: aws.String(draftsTableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: reqBody.ID},
		},
	}
	fmt.Printf("Deleting item from drafts table: %s\n", reqBody.ID)
	_, err = dbClient.DeleteItem(ctx, deleteItemInput)
	if err != nil {
		// 下書きの削除が失敗しても、投稿自体は成功しているので、ここではエラーを返さない（ログは出す）
		fmt.Printf("Error deleting item from drafts table: %v\n", err)
	}

	responseBody, _ := json.Marshal(map[string]string{"message": "Blog post published successfully!", "id": draftItem.ID})
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(responseBody),
	}, nil
}

func main() {
	lambda.Start(Handler)
}
