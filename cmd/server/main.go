package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	pb "github.com/plma-jshs/iam-server/proto"
	"google.golang.org/grpc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/authzed-go/v1"
	"github.com/authzed/grpcutil"
)

type iamServer struct {
	pb.UnimplementedIAMServiceServer
	client *authzed.Client
}

func Dissolve(text string) (string, string, error) {
	parts := strings.Split(text, ":")
	if len(parts) != 2 {
		return text, "", fmt.Errorf("invalid format")
	}
	return parts[0], parts[1], nil
}

func (s *iamServer) Read(ctx context.Context, req *pb.ReadRequest) (*pb.ReadResponse, error) {
	log.Printf("Read Request: %v", req)

	objectNamespace, objectId, err := Dissolve(req.Object)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Invalid object format: %v", err))
	}

	relation := req.Relation

	stream, err := s.client.ReadRelationships(ctx, &v1.ReadRelationshipsRequest{
		RelationshipFilter: &v1.RelationshipFilter{
			ResourceType:       objectNamespace,
			OptionalResourceId: objectId,
			OptionalRelation:   relation,
		},
	})
	if err != nil {
		return nil, err
	}

	var subjects []string

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		
		subjectId := resp.Relationship.Subject.Object.ObjectId
		subjects = append(subjects, subjectId)
	}

	return &pb.ReadResponse{Data: subjects}, nil
}

func (s *iamServer) Write(ctx context.Context, req *pb.WriteRequest) (*pb.WriteResponse, error) {
	log.Printf("Write Request: %v", req)

	objectNamespace, objectId, err := Dissolve(req.Object)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Invalid object format: %v", err))
	}
	subjectNamespace, subjectId, err := Dissolve(req.Subject)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Invalid subject format: %v", err))
	}

	relation := req.Relation

	_, err = s.client.WriteRelationships(ctx, &v1.WriteRelationshipsRequest{
		Updates: []*v1.RelationshipUpdate{
			{
				Operation: v1.RelationshipUpdate_OPERATION_TOUCH,
				Relationship: &v1.Relationship{
					Resource: &v1.ObjectReference{
						ObjectType: objectNamespace,
						ObjectId:   objectId,
					},
					Relation: relation,
					Subject: &v1.SubjectReference{
						Object: &v1.ObjectReference{
							ObjectType: subjectNamespace,
							ObjectId:   subjectId,
						},
					},
				},
			},
		},
	})

	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("Failed to write relation tuple: %v", err))
	}

	return &pb.WriteResponse{Success: true}, nil
}

func (s *iamServer) Check(ctx context.Context, req *pb.CheckRequest) (*pb.CheckResponse, error) {
	log.Printf("Check Request: %v", req)

	objectNamespace, objectId, err := Dissolve(req.Object)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Invalid object format: %v", err))
	}
	subjectNamespace, subjectId, err := Dissolve(req.Subject)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Invalid subject format: %v", err))
	}

	relation := req.Relation

	resp, err := s.client.CheckPermission(ctx, &v1.CheckPermissionRequest{
		Resource: &v1.ObjectReference{
			ObjectType: objectNamespace,
			ObjectId:   objectId,
		},
		Permission: relation,
		Subject: &v1.SubjectReference{
			Object: &v1.ObjectReference{
				ObjectType: subjectNamespace,
				ObjectId:   subjectId,
			},
		},
	})

	if err != nil {
		log.Fatalf("Error: %v", err)
		return nil, status.Error(codes.Internal, "Failed to check relation tuples")
	}

	return &pb.CheckResponse{Allowed: resp.Permissionship == v1.CheckPermissionResponse_PERMISSIONSHIP_HAS_PERMISSION}, nil
}

func setupSchema(client *authzed.Client) {
	ctx := context.Background()

	var files []string
	err := filepath.WalkDir("namespaces", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".zed") {
			files = append(files, path)
		}
		return nil
	})

	var mergedSchema strings.Builder
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("Failed to read file (%s): %v", file, err)
		}
		mergedSchema.Write(content)
		mergedSchema.WriteString("\n\n")
	}

	_, err = client.WriteSchema(ctx, &v1.WriteSchemaRequest{
		Schema: mergedSchema.String(),
	})
	if err != nil {
		log.Fatalf("Failed to write schema: %v", err)
	}

	fmt.Println("Schema setup completed successfully.")
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(".env file not found")
	}

	client, err := authzed.NewClient(
		os.Getenv("AUTHZED"),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpcutil.WithInsecureBearerToken(os.Getenv("SECRET_KEY")),
	)

	if err != nil {
		log.Fatalf("Failed to connect to SpiceDB: %v", err)
	}

	setupSchema(client)

	port := os.Getenv("PORT")
	lis, err := net.Listen("tcp", ":" + port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()

	pb.RegisterIAMServiceServer(s, &iamServer{client: client})

	log.Println("Server listening on port " + port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}