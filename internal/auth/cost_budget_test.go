package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCostBudgetManager_SetAndGetBudget(t *testing.T) {
	cbm := NewCostBudgetManager()
	userID := "test-user-1"

	// Set budget
	err := cbm.SetBudget(userID, 10.0, 100.0)
	assert.NoError(t, err)

	// Get budget
	budget, err := cbm.GetBudget(userID)
	assert.NoError(t, err)
	assert.Equal(t, userID, budget.UserID)
	assert.Equal(t, 10.0, budget.DailyBudget)
	assert.Equal(t, 100.0, budget.MonthlyBudget)
	assert.Equal(t, 0.0, budget.CurrentDayCost)
	assert.Equal(t, 0.0, budget.CurrentMonthCost)
}

func TestCostBudgetManager_RecordCost(t *testing.T) {
	cbm := NewCostBudgetManager()
	userID := "test-user-2"

	// Set budget
	err := cbm.SetBudget(userID, 10.0, 100.0)
	assert.NoError(t, err)

	// Record cost within budget
	err = cbm.RecordCost(userID, 5.0)
	assert.NoError(t, err)

	// Verify cost was recorded
	budget, err := cbm.GetBudget(userID)
	assert.NoError(t, err)
	assert.Equal(t, 5.0, budget.CurrentDayCost)
	assert.Equal(t, 5.0, budget.CurrentMonthCost)
	assert.Equal(t, 5.0, budget.TotalCost)
}

func TestCostBudgetManager_DailyBudgetExceeded(t *testing.T) {
	cbm := NewCostBudgetManager()
	userID := "test-user-3"

	// Set budget
	err := cbm.SetBudget(userID, 10.0, 100.0)
	assert.NoError(t, err)

	// Record cost that would exceed daily budget
	err = cbm.RecordCost(userID, 12.0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "daily budget exceeded")

	// Verify cost was not recorded
	budget, err := cbm.GetBudget(userID)
	assert.NoError(t, err)
	assert.Equal(t, 0.0, budget.CurrentDayCost)
}

func TestCostBudgetManager_MonthlyBudgetExceeded(t *testing.T) {
	cbm := NewCostBudgetManager()
	userID := "test-user-4"

	// Set budget with high daily limit but lower monthly limit
	err := cbm.SetBudget(userID, 200.0, 100.0)
	assert.NoError(t, err)

	// Record costs that stay within daily but exceed monthly
	err = cbm.RecordCost(userID, 40.0)
	assert.NoError(t, err)

	err = cbm.RecordCost(userID, 40.0)
	assert.NoError(t, err)

	// This should exceed monthly budget
	err = cbm.RecordCost(userID, 25.0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "monthly budget exceeded")

	// Verify total recorded cost
	budget, err := cbm.GetBudget(userID)
	assert.NoError(t, err)
	assert.Equal(t, 80.0, budget.CurrentMonthCost)
}

func TestCostBudgetManager_NoBudgetSet(t *testing.T) {
	cbm := NewCostBudgetManager()
	userID := "test-user-5"

	// Record cost without setting budget (should succeed)
	err := cbm.RecordCost(userID, 1000.0)
	assert.NoError(t, err)

	// Budget should not exist
	_, err = cbm.GetBudget(userID)
	assert.Error(t, err)
}

func TestCostBudgetManager_CheckBudget(t *testing.T) {
	cbm := NewCostBudgetManager()
	userID := "test-user-6"

	// Set budget
	err := cbm.SetBudget(userID, 10.0, 100.0)
	assert.NoError(t, err)

	// Check if cost is within budget
	err = cbm.CheckBudget(userID, 5.0)
	assert.NoError(t, err)

	// Check if cost would exceed daily budget
	err = cbm.CheckBudget(userID, 15.0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "would exceed daily budget")

	// Verify nothing was recorded
	budget, err := cbm.GetBudget(userID)
	assert.NoError(t, err)
	assert.Equal(t, 0.0, budget.CurrentDayCost)
}

func TestCostBudgetManager_UpdateBudget(t *testing.T) {
	cbm := NewCostBudgetManager()
	userID := "test-user-7"

	// Set initial budget
	err := cbm.SetBudget(userID, 10.0, 100.0)
	assert.NoError(t, err)

	// Record some cost
	err = cbm.RecordCost(userID, 5.0)
	assert.NoError(t, err)

	// Update budget (increase limits)
	err = cbm.SetBudget(userID, 20.0, 200.0)
	assert.NoError(t, err)

	// Verify budget was updated and costs preserved
	budget, err := cbm.GetBudget(userID)
	assert.NoError(t, err)
	assert.Equal(t, 20.0, budget.DailyBudget)
	assert.Equal(t, 200.0, budget.MonthlyBudget)
	assert.Equal(t, 5.0, budget.CurrentDayCost)
	assert.Equal(t, 5.0, budget.CurrentMonthCost)
}

func TestCostBudgetManager_ListBudgets(t *testing.T) {
	cbm := NewCostBudgetManager()

	// Create multiple budgets
	cbm.SetBudget("user1", 10.0, 100.0)
	cbm.SetBudget("user2", 20.0, 200.0)
	cbm.SetBudget("user3", 30.0, 300.0)

	// List all budgets
	budgets := cbm.ListBudgets()
	assert.Len(t, budgets, 3)
}

func TestCostBudgetManager_DeleteBudget(t *testing.T) {
	cbm := NewCostBudgetManager()
	userID := "test-user-8"

	// Set budget
	err := cbm.SetBudget(userID, 10.0, 100.0)
	assert.NoError(t, err)

	// Delete budget
	err = cbm.DeleteBudget(userID)
	assert.NoError(t, err)

	// Verify budget is gone
	_, err = cbm.GetBudget(userID)
	assert.Error(t, err)
}

func TestCostBudgetManager_ResetDailyBudgets(t *testing.T) {
	cbm := NewCostBudgetManager()
	userID := "test-user-9"

	// Set budget and record cost
	cbm.SetBudget(userID, 10.0, 100.0)
	cbm.RecordCost(userID, 5.0)

	// Manually set CurrentDay to yesterday
	cbm.mu.Lock()
	budget := cbm.budgets[userID]
	budget.CurrentDay = time.Now().AddDate(0, 0, -1)
	cbm.mu.Unlock()

	// Reset daily budgets
	cbm.ResetDailyBudgets()

	// Verify daily cost was reset but monthly was not
	budget, _ = cbm.GetBudget(userID)
	assert.Equal(t, 0.0, budget.CurrentDayCost)
	assert.Equal(t, 5.0, budget.CurrentMonthCost)
}

func TestCostBudgetManager_ResetMonthlyBudgets(t *testing.T) {
	cbm := NewCostBudgetManager()
	userID := "test-user-10"

	// Set budget and record cost
	cbm.SetBudget(userID, 50.0, 100.0)
	cbm.RecordCost(userID, 30.0)

	// Manually set CurrentMonth to last month
	cbm.mu.Lock()
	budget := cbm.budgets[userID]
	budget.CurrentMonth = time.Now().AddDate(0, -1, 0)
	cbm.mu.Unlock()

	// Reset monthly budgets
	cbm.ResetMonthlyBudgets()

	// Verify monthly cost was reset
	budget, _ = cbm.GetBudget(userID)
	assert.Equal(t, 0.0, budget.CurrentMonthCost)
	assert.Equal(t, 30.0, budget.TotalCost) // Total should be preserved
}

func TestHelperFunctions(t *testing.T) {
	now := time.Now()

	// Test startOfDay
	startDay := startOfDay(now)
	assert.Equal(t, 0, startDay.Hour())
	assert.Equal(t, 0, startDay.Minute())
	assert.Equal(t, 0, startDay.Second())

	// Test startOfMonth
	startMonth := startOfMonth(now)
	assert.Equal(t, 1, startMonth.Day())
	assert.Equal(t, 0, startMonth.Hour())

	// Test isSameDay
	tomorrow := now.AddDate(0, 0, 1)
	assert.True(t, isSameDay(now, now))
	assert.False(t, isSameDay(now, tomorrow))

	// Test isSameMonth
	nextMonth := now.AddDate(0, 1, 0)
	assert.True(t, isSameMonth(now, now))
	assert.False(t, isSameMonth(now, nextMonth))
}

func TestCostBudgetManager_ZeroBudget(t *testing.T) {
	cbm := NewCostBudgetManager()
	userID := "test-user-11"

	// Set budget with zero values (unlimited)
	err := cbm.SetBudget(userID, 0.0, 0.0)
	assert.NoError(t, err)

	// Record large cost (should succeed with zero budget = unlimited)
	err = cbm.RecordCost(userID, 1000.0)
	assert.NoError(t, err)

	// Verify cost was recorded
	budget, err := cbm.GetBudget(userID)
	assert.NoError(t, err)
	assert.Equal(t, 1000.0, budget.CurrentDayCost)
}

func TestCostBudgetManager_MultipleSmallCosts(t *testing.T) {
	cbm := NewCostBudgetManager()
	userID := "test-user-12"

	// Set budget
	err := cbm.SetBudget(userID, 10.0, 100.0)
	assert.NoError(t, err)

	// Record multiple small costs
	for i := 0; i < 5; i++ {
		err = cbm.RecordCost(userID, 1.5)
		assert.NoError(t, err)
	}

	// Verify total cost
	budget, err := cbm.GetBudget(userID)
	assert.NoError(t, err)
	assert.Equal(t, 7.5, budget.CurrentDayCost)
	assert.Equal(t, 7.5, budget.CurrentMonthCost)

	// Next cost should exceed daily budget
	err = cbm.RecordCost(userID, 3.0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "daily budget exceeded")
}
