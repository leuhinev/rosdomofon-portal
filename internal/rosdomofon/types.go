package rosdomofon

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type Entrance struct {
	Country  Country `json:"country"`
	City     string  `json:"city"`
	Street   Street  `json:"street"`
	House    House   `json:"house"`
	Entrance struct {
		ID                   int           `json:"id"`
		Number               string        `json:"number"`
		FlatStart            int           `json:"flatStart"`
		FlatEnd              int           `json:"flatEnd"`
		AdditionalFlatRanges []interface{} `json:"additionalFlatRanges"`
		Prefix               string        `json:"prefix"`
	} `json:"entrance"`
}

type Country struct {
	ShortName string `json:"shortName"`
	Name      string `json:"name"`
}

type House struct {
	ID     int    `json:"id"`
	Number string `json:"number"`
}

type Street struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	CodeKladr string `json:"codeKladr"`
	CodeFias  string `json:"codeFias"`
}

type Flat struct {
	ID      int     `json:"id"`
	Owner   Owner   `json:"owner"`
	Address Address `json:"address"`
	Virtual bool    `json:"virtual"`
}

type Owner struct {
	ID          int          `json:"id"`
	Phone       int64        `json:"phone"`
	Delegations []Delegation `json:"delegations"`
}

type Delegation struct {
	ID          int     `json:"id"`
	FromAbonent Abonent `json:"fromAbonent"`
	ToAbonent   Abonent `json:"toAbonent"`
}

type Abonent struct {
	ID    int   `json:"id,omitempty"`
	Phone int64 `json:"phone"`
}

type Address struct {
	Country  Country       `json:"country"`
	City     string        `json:"city"`
	Street   Street        `json:"street"`
	House    House         `json:"house"`
	Entrance EntranceInner `json:"entrance"`
	Flat     int           `json:"flat"`
}

type EntranceInner struct {
	ID                   int           `json:"id"`
	Number               string        `json:"number"`
	FlatStart            int           `json:"flatStart"`
	FlatEnd              int           `json:"flatEnd"`
	AdditionalFlatRanges []interface{} `json:"additionalFlatRanges"`
}

type MessageRequest struct {
	ToAbonents     []MessageAbonent `json:"toAbonents"`
	Channel        string           `json:"channel"`
	Message        string           `json:"message"`
	DeliveryMethod string           `json:"deliveryMethod"`
}

type MessageAbonent struct {
	Phone string `json:"phone"`
}

type MessageResponse struct {
	ID      int64  `json:"id"`
	Success bool   `json:"success"`
	Result  string `json:"result"`
}

// ActionTokenInfo - ответ от API при проверке токена из WebView
type ActionTokenInfo struct {
	ID           int    `json:"id"`
	Token        string `json:"token"`
	UseCount     int    `json:"useCount"`
	ExpiryDate   int64  `json:"expiryDate"`
	SubscriberId int    `json:"subscriberId"`
}

// Connection - структура для /connections endpoint
type Connection struct {
	ID      int     `json:"id"`
	Account Account `json:"account"`
	Flat    Flat    `json:"flat"`
	Blocked bool    `json:"blocked"`
}

type Account struct {
	ID      int   `json:"id"`
	Owner   Owner `json:"owner"`
	Blocked bool  `json:"blocked"`
}

// EntranceService - структура для парсинга services из /entrances
type EntranceService struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// EntranceResponse - структура для /entrances
type EntranceResponse struct {
	Content []EntranceItem `json:"content"`
}

type EntranceItem struct {
	ID       int               `json:"id"`
	Services []EntranceService `json:"services"`
}

// ServiceInfo - информация о сервисе для хранения в памяти
type ServiceInfo struct {
	ID   int
	Type string
}

// AddressComponents - стабильные компоненты адреса
type AddressComponents struct {
	StreetID   int
	HouseID    int
	EntranceID int
	FlatNumber int
	AddressStr string
}

// AbonentFlat - квартира абонента из /abonents/{subscriberId}/flats
type AbonentFlat struct {
	Address            Address `json:"address"`
	ID                 int     `json:"id"`
	AccountId          int     `json:"accountId"`
	CameraId           int     `json:"cameraId,omitempty"`
	RdaUid             string  `json:"rdaUid,omitempty"`
	SoftwareIntercomId int     `json:"softwareIntercomId,omitempty"`
	HardwareIntercomId int     `json:"hardwareIntercomId,omitempty"`
	Owner              struct {
		ID int `json:"id"`
	} `json:"owner"`
	Virtual bool `json:"virtual"`
}
