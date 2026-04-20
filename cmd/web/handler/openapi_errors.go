package handler

type errCreateBadRequest struct {
	Code    string `json:"code"    example:"bad_request"`
	Message string `json:"message" example:"amount must be greater than zero"`
}

type errCreateNotFound struct {
	Code    string `json:"code"    example:"not_found"`
	Message string `json:"message" example:"source user not found"`
}

type errInsufficientBalance struct {
	Code    string `json:"code"    example:"conflict"`
	Message string `json:"message" example:"insufficient balance"`
}

type errListBadRequest struct {
	Code    string `json:"code"    example:"bad_request"`
	Message string `json:"message" example:"invalid user_id"`
}

type errUserNotFound struct {
	Code    string `json:"code"    example:"not_found"`
	Message string `json:"message" example:"user not found"`
}

type errTransitionBadRequest struct {
	Code    string `json:"code"    example:"bad_request"`
	Message string `json:"message" example:"invalid transaction id"`
}

type errTransactionNotFound struct {
	Code    string `json:"code"    example:"not_found"`
	Message string `json:"message" example:"transaction not found"`
}

type errTransactionNotPending struct {
	Code    string `json:"code"    example:"conflict"`
	Message string `json:"message" example:"transaction is not pending"`
}

type errInternalServer struct {
	Code    string `json:"code"    example:"internal_server_error"`
	Message string `json:"message" example:"internal server error"`
}
