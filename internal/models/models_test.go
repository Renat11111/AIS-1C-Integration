package models

import (
	"net/http"
	"testing"
)

func TestAISRequest_Validate(t *testing.T) {
	tests := []struct {
		name       string
		request    AISRequest
		httpMethod string
		wantErr    bool
	}{
		{
			name: "Valid Sale (POST)",
			request: AISRequest{
				ID: "msg-1",
				Data: AISDocument{
					SaleId: "S-1",
				},
			},
			httpMethod: http.MethodPost,
			wantErr:    false,
		},
		{
			name: "Missing Envelope ID",
			request: AISRequest{
				Data: AISDocument{
					SaleId: "S-1",
				},
			},
			httpMethod: http.MethodPost,
			wantErr:    true,
		},
		{
			name: "Update (PUT) missing SaleId",
			request: AISRequest{
				ID: "msg-1",
				Data: AISDocument{
					SaleComment: "Updated",
				},
			},
			httpMethod: http.MethodPut,
			wantErr:    true,
		},
		{
			name: "Cancellation (DELETE) missing SaleDate",
			request: AISRequest{
				ID: "msg-1",
				Data: AISDocument{
					SaleId:      "S-1",
					UserId:      "U-1",
					SaleComment: "Reason",
				},
			},
			httpMethod: http.MethodDelete,
			wantErr:    true,
		},
		{
			name: "Valid Cancellation (DELETE)",
			request: AISRequest{
				ID: "msg-1",
				Data: AISDocument{
					SaleId:      "S-1",
					SaleDate:    "2023-01-01",
					UserId:      "U-1",
					SaleComment: "Client request",
				},
			},
			httpMethod: http.MethodDelete,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate(tt.httpMethod)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
