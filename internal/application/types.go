package application

import "time"

type ElectricityConsumption struct {
	MeasurementDate time.Time
	ConsumptionKWh  float64
	TotalAmount     float64
}

type ElectricitySummary struct {
	TotalKWh    float64
	TotalAmount float64
	Data        []*ElectricityConsumption
}
