package helper

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func GetAndValidateTypeSafeRequest(c *gin.Context) (*dto.TypeSafeRequest, error) {
	var request dto.TypeSafeRequest
	if err := common.UnmarshalBodyReusable(c, &request); err != nil {
		return nil, err
	}
	if err := ValidateTypeSafeRequest(&request); err != nil {
		return nil, err
	}
	return &request, nil
}

func ValidateTypeSafeRequest(request *dto.TypeSafeRequest) error {
	if request == nil || strings.TrimSpace(request.Model) == "" {
		return errors.New("model is required")
	}
	if !typeSafeStructuredValue(request.State) {
		return errors.New("state must be a string, object, or array")
	}
	if len(request.Questions) == 0 {
		return errors.New("questions must be a non-empty object")
	}
	for key, question := range request.Questions {
		if !typeSafeStructuredValue(question.Instructions) {
			return fmt.Errorf("questions.%s.instructions must be a string, object, or array", key)
		}
		criteria := gjson.ParseBytes(question.Criteria)
		switch question.Type {
		case "noul":
			if len(question.Criteria) == 0 {
				continue
			}
			if !criteria.IsObject() {
				return fmt.Errorf("questions.%s.criteria must be an object", key)
			}
			for name, value := range criteria.Map() {
				if (name != "true" && name != "false") || value.Type != gjson.String {
					return fmt.Errorf("questions.%s.criteria must contain string descriptions for true or false", key)
				}
			}
		case "choice":
			if !criteria.IsObject() || len(criteria.Map()) == 0 {
				return fmt.Errorf("questions.%s.criteria must be a non-empty object", key)
			}
			for _, value := range criteria.Map() {
				if value.Type != gjson.String && value.Type != gjson.Null {
					return fmt.Errorf("questions.%s.criteria descriptions must be strings or null", key)
				}
			}
		case "score":
			if !criteria.IsArray() || len(criteria.Array()) < 2 {
				return fmt.Errorf("questions.%s.criteria must have at least two levels", key)
			}
			for _, value := range criteria.Array() {
				if value.Type != gjson.String {
					return fmt.Errorf("questions.%s.criteria levels must be strings", key)
				}
			}
		default:
			return fmt.Errorf("questions.%s.type must be noul, choice, or score", key)
		}
	}
	return nil
}

func typeSafeStructuredValue(raw []byte) bool {
	if !gjson.ValidBytes(raw) {
		return false
	}
	value := gjson.ParseBytes(raw)
	return value.Type == gjson.String || value.IsObject() || value.IsArray()
}
