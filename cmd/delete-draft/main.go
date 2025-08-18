package main

import (
	"context"
	"fmt"
	"os"
	"encoding/json"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var dbClient *dynamodb.Client
var draftsTableName = os.Getenv("DRAFTS_TABLE_NAME") // 下書きテーブル名

func init() {
	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading AWS config: %v\n", err)
	}
	// Create a DynamoDB client
	dbClient = dynamodb.NewFromConfig(cfg)
}

// Handler handles the API Gateway proxy request to delete a draft.
func Handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	fmt.Println("Received request for delete draft handler.")

	// Get the draft ID from the path parameters
	// API Gatewayはパスパラメータをrequest.PathParameters["id"]にマッピングします
	draftID := request.PathParameters["id"]
	if draftID == "" {
		fmt.Println("Error: Missing draft ID in path parameters.")
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: "Missing draft ID"}, nil
	}

	// Create DeleteItemInput for DynamoDB
	deleteItemInput := &dynamodb.DeleteItemInput{
		TableName: aws.String(draftsTableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: draftID},
		},
	}

	// Delete the item from the DynamoDB drafts table
	fmt.Printf("Deleting item with ID: %s from drafts table: %s\n", draftID, draftsTableName)
	_, err := dbClient.DeleteItem(ctx, deleteItemInput)
	if err != nil {
		fmt.Printf("Error deleting item from DynamoDB: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Failed to delete draft: %v", err)}, nil
	}

	// Return a success response
	responseBody, _ := json.Marshal(map[string]string{"message": fmt.Sprintf("Draft with ID %s deleted successfully", draftID)})
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(responseBody),
	}, nil
}

func main() {
	lambda.Start(Handler)
}
