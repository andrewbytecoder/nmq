package model

type ProductInfo struct {
	ProductID               string `gorm:"column:product_id;primaryKey" json:"product_id"`
	ProductName             string `gorm:"column:product_name" json:"product_name"`
	ProductDesc             string `gorm:"column:product_desc" json:"product_desc"`
	ProductType             string `gorm:"column:product_type" json:"product_type"`
	ProductStatus           string `gorm:"column:product_status" json:"product_status"`
	ProductVersion          string `gorm:"column:product_version" json:"product_version"`
	ProductIcon             string `gorm:"column:product_icon" json:"product_icon"`
	ProductLogo             string `gorm:"column:product_logo" json:"product_logo"`
	ProductBanner           string `gorm:"column:product_banner" json:"product_banner"`
	ProductHome             string `gorm:"column:product_home" json:"product_home"`
	ProductDoc              string `gorm:"column:product_doc" json:"product_doc"`
	ProductDocUrl           string `gorm:"column:product_doc_url" json:"product_doc_url"`
	ProductDocType          string `gorm:"column:product_doc_type" json:"product_doc_type"`
	ProductDocSize          string `gorm:"column:product_doc_size" json:"product_doc_size"`
	ProductDocMd5           string `gorm:"column:product_doc_md5" json:"product_doc_md5"`
	ProductDocDownloadUrl   string `gorm:"column:product_doc_download_url" json:"product_doc_download_url"`
	ProductDocDownloadCount int    `gorm:"column:product_doc_download_count" json:"product_doc_download_count"`
}

// TableName specifies the table name for GORM.
func (ProductInfo) TableName() string {
	return "product_info"
}

type ProductInfoMng interface {
	GetAllProductInfo() ([]ProductInfo, error)
	SaveProductInfo(productInfo ProductInfo) error
	SaveProductInfoList(productInfoList []ProductInfo) error
}
