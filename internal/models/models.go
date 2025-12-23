package models

import (
	"errors"
	"net/http"
)

// AISRequest - обертка запроса (конверт).
type AISRequest struct {
	ID        string      `json:"id"`
	Method    string      `json:"method"` // sale, sale_update, sale_cancellation, etc.
	Timestamp int64       `json:"timestamp"`
	Data      AISDocument `json:"data"`
}

// Validate проверяет обязательные поля в зависимости от метода.
// Если methodOverride не пуст, он используется вместо req.Method (полезно при переопределении из HTTP метода)
func (r *AISRequest) Validate(httpMethod string) error {
	if r.ID == "" {
		return errors.New("missing 'id' field (envelope ID)")
	}

	// Общая проверка на наличие SaleId (для POST/PUT/DELETE)
	if r.Data.SaleId == nil || r.Data.SaleId == "" {
		return errors.New("missing 'data.SaleId'")
	}

	// Специфичная валидация для ОТМЕНЫ (DELETE)
	if httpMethod == http.MethodDelete {
		if r.Data.SalePayStatusId == nil || r.Data.SalePayStatusId == "" {
			return errors.New("cancellation requires 'data.SalePayStatusId'")
		}
	}

	return nil
}

// SaleDetail - Детали продажи.
type SaleDetail struct {
	SaleDetailId          string  `json:"SaleDetailId"`
	SaleDetailSaleId      string  `json:"SaleDetailSaleId"`
	SaleDetailServiceId   string  `json:"SaleDetailServiceId"`
	SaleDetailQuantity    float64 `json:"SaleDetailQuantity"`
	SaleDetailPrice       float64 `json:"SaleDetailPrice"`
	SaleDetailAmount      float64 `json:"SaleDetailAmount"`
	SaleDetailVATRate     float64 `json:"SaleDetailVATRate"`
	SaleDetailVATAmount   float64 `json:"SaleDetailVATAmount"`
	SaleDetailTotalAmount float64 `json:"SaleDetailTotalAmount"`
	
	ServiceName           string  `json:"ServiceName,omitempty"`
	ServiceCode           string  `json:"ServiceCode,omitempty"`
	ServiceUnit           string  `json:"ServiceUnit,omitempty"`
}

// AISDocument - Основная структура.
type AISDocument struct {
	// --- Sale Fields ---
	SaleId            interface{} `json:"SaleId"`
	SaleClientId      string      `json:"SaleClientId"`
	SalePrecinctId    string      `json:"SalePrecinctId"`
	SaleInspectorId   string      `json:"SaleInspectorId"`
	
	// Enum: 1=NotPaid, 2=Paid, 3=Partial, 4=Cancelled, 5=Pending, 6=Return, etc.
	SalePayStatusId   interface{} `json:"SalePayStatusId"` 
	
	SaleDate          string  `json:"SaleDate"` // Format: ISO8601 or YYYY-MM-DD HH:mm:ss
	SaleDeclarationNo string  `json:"SaleDeclarationNo"`
	SaleComment       string  `json:"SaleComment"` // Required for Cancellation (DELETE)
	SaleIsDiplomatic  bool    `json:"SaleIsDiplomatic"`
	SaleCarNumber     string  `json:"SaleCarNumber"`
	
	// Enum: 0=None, 1=Avans, 3=Hop, 4=Vais, 5=Contract, 6=Manual, 7=Emanat, 8=Kocurme
	SalePaymentType   string  `json:"SalePaymentType"`
	
	SalePayDate       string  `json:"SalePayDate"`
	SalePayType       string  `json:"SalePayType"`
	SaleMainAmount    float64 `json:"SaleMainAmount"`
	SaleVatAmount     float64 `json:"SaleVatAmount"`
	Privilege         string  `json:"Privilege"`

	// --- Array of Nomenclatures ---
	Details []SaleDetail `json:"SaleDetails"`

	// --- Client Fields ---
	// Required: Id, Type, Voen, Name. Others are optional.
	ClientId               string `json:"ClientId"`
	
	// Enum: 1=Legal, 2=Resident, 3=NonResident
	ClientClientTypeId     string `json:"ClientClientTypeId"`
	
	ClientVOENOrPassportNo string `json:"ClientVOENOrPassportNo"`
	ClientFullName         string `json:"ClientFullName"`
	
	ClientEmail            string `json:"ClientEmail,omitempty"`
	ClientPhone            string `json:"ClientPhone,omitempty"`
	ClientFieldOfActivity  string `json:"ClientFieldOfActivity,omitempty"`
	ClientCity             string `json:"ClientCity,omitempty"`
	ClientLegalAddress     string `json:"ClientLegalAddress,omitempty"`
	ClientActualAddress    string `json:"ClientActualAddress,omitempty"`
	ClientIndex            string `json:"ClientIndex,omitempty"`
	ClientWebsite          string `json:"ClientWebsite,omitempty"`
	ClientLeaderFullName   string `json:"ClientLeaderFullName,omitempty"`
	ClientLeaderDuty       string `json:"ClientLeaderDuty,omitempty"`
	ClientContractorTypeId string `json:"ClientContractorTypeId,omitempty"`

	// --- User Fields ---
	// Required for Cancellation: UserId
	UserId               string `json:"UserId"`
	UserPrecinctId       string `json:"UserPrecinctId"`
	
	// Enum: 1=SuperAdmin, 2=Accountant, 3=Inspector, ...
	UserStatusId         string `json:"UserStatusId"`
	
	UserRegistrationDate string `json:"UserRegistrationDate"`
	UserName             string `json:"UserName"`
	UserFirstName        string `json:"UserFirstName"`
	UserLastName         string `json:"UserLastName"`
	UserEmail            string `json:"UserEmail"`
	UserPhone            string `json:"UserPhone"`
	UserActive           bool   `json:"UserActive"`

	// --- Contract Fields ---
	// All required except EndDate
	ContractId         string `json:"ContractId"`
	ContractNo         string `json:"ContractNo"`
	ContractSignedDate string `json:"ContractSignedDate"`
	ContractStartDate  string `json:"ContractStartDate"`
	ContractEndDate    string `json:"ContractEndDate,omitempty"` // Optional
	ContractType       string `json:"ContractType"`
	ContractStatus     string `json:"ContractStatus"`
	ContractIsTransfer bool   `json:"ContractIsTransfer"`
	ContractPayTime    string `json:"ContractPayTime"`

	// --- Reference Fields ---
	PrecinctName   string `json:"PrecinctName,omitempty"`
	DepartmentName string `json:"DepartmentName,omitempty"`
	PayStatusName  string `json:"PayStatusName,omitempty"`
}

// APIResponse structure.
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Error   string      `json:"error,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}
