package models

// AISRequest - обертка запроса (конверт).
type AISRequest struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"`      // Тип документа: "sale", "return" и т.д.
	Timestamp int64       `json:"timestamp"`
	Data      AISDocument `json:"data"`      // Основные данные документа
	
	// Internal field to pass HTTP method to worker (POST/PUT/DELETE)
	Method    string      `json:"-"` 
}

// SaleDetail - Детали продажи (Номенклатура), приходят в виде массива.
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
	
	// Дополнительные поля по услуге
	ServiceName           string  `json:"ServiceName,omitempty"`
	ServiceCode           string  `json:"ServiceCode,omitempty"`
	ServiceUnit           string  `json:"ServiceUnit,omitempty"`
}

// AISDocument - Полная структура данных.
type AISDocument struct {
	// --- Sale Fields ---
	SaleId            string  `json:"SaleId"`
	SaleClientId      string  `json:"SaleClientId"`
	SalePrecinctId    string  `json:"SalePrecinctId"`
	SaleInspectorId   string  `json:"SaleInspectorId"`
	SalePayStatusId   string  `json:"SalePayStatusId"`
	SaleDate          string  `json:"SaleDate"`
	SaleDeclarationNo string  `json:"SaleDeclarationNo"`
	SaleComment       string  `json:"SaleComment"`
	SaleIsDiplomatic  bool    `json:"SaleIsDiplomatic"`
	SaleCarNumber     string  `json:"SaleCarNumber"`
	SalePaymentType   string  `json:"SalePaymentType"`
	SalePayDate       string  `json:"SalePayDate"`
	SalePayType       string  `json:"SalePayType"`
	SaleMainAmount    float64 `json:"SaleMainAmount"`
	SaleVatAmount     float64 `json:"SaleVatAmount"`
	Privilege         string  `json:"Privilege"`

	// --- Array of Nomenclatures ---
	Details []SaleDetail `json:"SaleDetails"`

	// --- Client Fields ---
	ClientId               string `json:"ClientId"`
	ClientClientTypeId     string `json:"ClientClientTypeId"`
	ClientVOENOrPassportNo string `json:"ClientVOENOrPassportNo"`
	ClientFullName         string `json:"ClientFullName"`
	ClientEmail            string `json:"ClientEmail"`
	ClientPhone            string `json:"ClientPhone"`
	ClientFieldOfActivity  string `json:"ClientFieldOfActivity"`
	ClientCity             string `json:"ClientCity"`
	ClientLegalAddress     string `json:"ClientLegalAddress"`
	ClientActualAddress    string `json:"ClientActualAddress"`
	ClientIndex            string `json:"ClientIndex"`
	ClientWebsite          string `json:"ClientWebsite"`
	ClientLeaderFullName   string `json:"ClientLeaderFullName"`
	ClientLeaderDuty       string `json:"ClientLeaderDuty"`
	ClientContractorTypeId string `json:"ClientContractorTypeId"`

	// --- User Fields ---
	UserId               string `json:"UserId"`
	UserPrecinctId       string `json:"UserPrecinctId"`
	UserStatusId         string `json:"UserStatusId"`
	UserRegistrationDate string `json:"UserRegistrationDate"`
	UserName             string `json:"UserName"`
	UserFirstName        string `json:"UserFirstName"`
	UserLastName         string `json:"UserLastName"`
	UserEmail            string `json:"UserEmail"`
	UserPhone            string `json:"UserPhone"`
	UserActive           bool   `json:"UserActive"`

	// --- Contract Fields ---
	ContractId         string `json:"ContractId,omitempty"`
	ContractNo         string `json:"ContractNo,omitempty"`
	ContractSignedDate string `json:"ContractSignedDate,omitempty"`
	ContractStartDate  string `json:"ContractStartDate,omitempty"`
	ContractEndDate    string `json:"ContractEndDate,omitempty"`
	ContractType       string `json:"ContractType,omitempty"`
	ContractStatus     string `json:"ContractStatus,omitempty"`
	ContractIsTransfer bool   `json:"ContractIsTransfer,omitempty"`
	ContractPayTime    string `json:"ContractPayTime,omitempty"`

	// --- Transaction Fields ---
	TransactionId                string  `json:"TransactionId,omitempty"`
	TransactionAmount            float64 `json:"TransactionAmount,omitempty"`
	TransactionComment           string  `json:"TransactionComment,omitempty"`
	TransactionDate              string  `json:"TransactionDate,omitempty"`
	TransactionBalanceBeforeSale float64 `json:"TransactionBalanceBeforeSale,omitempty"`
	TransactionIsSale            bool    `json:"TransactionIsSale,omitempty"`

	// --- Reference Fields ---
	PrecinctName   string `json:"PrecinctName,omitempty"`
	DepartmentName string `json:"DepartmentName,omitempty"`
	PayStatusName  string `json:"PayStatusName,omitempty"`
}

// APIResponse is the standard response structure.
type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}
