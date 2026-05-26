package models

import "github.com/go-playground/validator/v10"

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"  validate:"required,min=2"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age"   validate:"gte=0,lte=130"`
}

// CustomValidator implements echo.Validator so Echo can call c.Validate().
type CustomValidator struct {
	V *validator.Validate
}

func (cv *CustomValidator) Validate(i any) error {
	return cv.V.Struct(i)
}
