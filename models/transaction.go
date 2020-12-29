package models

import (
	"gorm.io/gorm"
)

const (
	TRANSACTION_STATUS_NEW = "new"
	TRANSACTION_STATUS_PENDING = "pending"
	TRANSACTION_STATUS_COMPLETE = "complete"
	TRANSACTION_STATUS_REJECT = "reject"
)

type Transaction struct {
	gorm.Model
	//
	Amount float64 `sql:"type:decimal(8,2);"`
	Status string
	//
	Payment string // JSON: {"Method": "stripe", "Id": "pi_1HuDdsLxvolFmsmRDXUNRZdj"}
	//				  JSON: {"Method": "mollie", "Id": "ord_wqqie0"}
	//
	Order *Order `gorm:"foreignKey:OrderId"`
	OrderId uint
}

type TransactionPayment struct {
	Mollie *TransactionPaymentMollie `json:",omitempty"`
	Stripe *TransactionPaymentStripe `json:",omitempty"`
}

type TransactionPaymentMollie struct {
	Id string `json:",omitempty"`
	Error string `json:",omitempty"`
}

type TransactionPaymentStripe struct {
	Id string `json:",omitempty"`
	Error string `json:",omitempty"`
}

func GetTransactionsByOrderId(connector *gorm.DB, id int) ([]*Transaction, error) {
	db := connector
	var transactions []*Transaction
	db.Model(&Transaction{}).Where("order_id = ?", id).Find(&transactions)
	if err :=  db.Error; err != nil {
		return nil, err
	}
	return transactions, nil
}

func CreateTransaction(connector *gorm.DB, transaction *Transaction) (uint, error) {
	db := connector
	db.Debug().Create(&transaction)
	if err := db.Error; err != nil {
		return 0, err
	}
	return transaction.ID, nil
}

func GetTransaction(connector *gorm.DB, id int) (*Transaction, error) {
	db := connector
	var transaction Transaction
	db.Debug().Find(&transaction, id)
	if err := db.Error; err != nil {
		return nil, err
	}
	return &transaction, nil
}

func UpdateTransaction(connector *gorm.DB, transaction *Transaction) error {
	db := connector
	db.Debug().Save(&transaction)
	return db.Error
}

func DeleteTransaction(connector *gorm.DB, transaction *Transaction) error {
	db := connector
	db.Debug().Unscoped().Delete(&transaction)
	return db.Error
}