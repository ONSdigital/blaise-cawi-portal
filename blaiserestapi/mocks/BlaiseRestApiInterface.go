// Code generated by mockery v2.10.0. DO NOT EDIT.

package mocks

import (
	blaiserestapi "github.com/ONSdigital/blaise-cawi-portal/blaiserestapi"
	mock "github.com/stretchr/testify/mock"
)

// BlaiseRestApiInterface is an autogenerated mock type for the BlaiseRestApiInterface type
type BlaiseRestApiInterface struct {
	mock.Mock
}

// GetInstrumentSettings provides a mock function with given fields: _a0
func (_m *BlaiseRestApiInterface) GetInstrumentSettings(_a0 string) (blaiserestapi.InstrumentSettings, error) {
	ret := _m.Called(_a0)

	var r0 blaiserestapi.InstrumentSettings
	if rf, ok := ret.Get(0).(func(string) blaiserestapi.InstrumentSettings); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(blaiserestapi.InstrumentSettings)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
