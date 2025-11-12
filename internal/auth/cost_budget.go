package auth

import (
	"fmt"
	"sync"
	"time"
)

// CostBudget tracks cost usage for a user
type CostBudget struct {
	UserID            string    `json:"user_id"`
	DailyBudget       float64   `json:"daily_budget"`        // USD
	MonthlyBudget     float64   `json:"monthly_budget"`      // USD
	CurrentDayCost    float64   `json:"current_day_cost"`    // USD
	CurrentMonthCost  float64   `json:"current_month_cost"`  // USD
	CurrentDay        time.Time `json:"current_day"`         // Date for current day tracking
	CurrentMonth      time.Time `json:"current_month"`       // Date for current month tracking
	TotalCost         float64   `json:"total_cost"`          // Total cost all time
	LastResetDaily    time.Time `json:"last_reset_daily"`    // Last daily reset
	LastResetMonthly  time.Time `json:"last_reset_monthly"`  // Last monthly reset
}

// CostBudgetManager manages cost budgets for all users
type CostBudgetManager struct {
	budgets map[string]*CostBudget // userID -> CostBudget
	mu      sync.RWMutex
}

// NewCostBudgetManager creates a new cost budget manager
func NewCostBudgetManager() *CostBudgetManager {
	return &CostBudgetManager{
		budgets: make(map[string]*CostBudget),
	}
}

// SetBudget sets or updates budget limits for a user
func (cbm *CostBudgetManager) SetBudget(userID string, dailyBudget, monthlyBudget float64) error {
	cbm.mu.Lock()
	defer cbm.mu.Unlock()

	now := time.Now()

	// Get existing budget or create new one
	budget, exists := cbm.budgets[userID]
	if !exists {
		budget = &CostBudget{
			UserID:           userID,
			CurrentDay:       startOfDay(now),
			CurrentMonth:     startOfMonth(now),
			LastResetDaily:   now,
			LastResetMonthly: now,
		}
		cbm.budgets[userID] = budget
	}

	budget.DailyBudget = dailyBudget
	budget.MonthlyBudget = monthlyBudget

	return nil
}

// GetBudget retrieves budget information for a user
func (cbm *CostBudgetManager) GetBudget(userID string) (*CostBudget, error) {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	budget, exists := cbm.budgets[userID]
	if !exists {
		return nil, fmt.Errorf("budget not found for user: %s", userID)
	}

	// Create a copy to avoid race conditions
	budgetCopy := *budget
	return &budgetCopy, nil
}

// RecordCost records a cost for a user and checks if budget is exceeded
func (cbm *CostBudgetManager) RecordCost(userID string, cost float64) error {
	cbm.mu.Lock()
	defer cbm.mu.Unlock()

	budget, exists := cbm.budgets[userID]
	if !exists {
		// No budget set - allow by default
		return nil
	}

	now := time.Now()

	// Check if we need to reset daily costs
	if !isSameDay(budget.CurrentDay, now) {
		budget.CurrentDay = startOfDay(now)
		budget.CurrentDayCost = 0
		budget.LastResetDaily = now
	}

	// Check if we need to reset monthly costs
	if !isSameMonth(budget.CurrentMonth, now) {
		budget.CurrentMonth = startOfMonth(now)
		budget.CurrentMonthCost = 0
		budget.LastResetMonthly = now
	}

	// Calculate new costs
	newDayCost := budget.CurrentDayCost + cost
	newMonthCost := budget.CurrentMonthCost + cost

	// Check daily budget
	if budget.DailyBudget > 0 && newDayCost > budget.DailyBudget {
		return fmt.Errorf("daily budget exceeded: %.4f/%.4f USD (this request: %.4f USD)",
			budget.CurrentDayCost, budget.DailyBudget, cost)
	}

	// Check monthly budget
	if budget.MonthlyBudget > 0 && newMonthCost > budget.MonthlyBudget {
		return fmt.Errorf("monthly budget exceeded: %.4f/%.4f USD (this request: %.4f USD)",
			budget.CurrentMonthCost, budget.MonthlyBudget, cost)
	}

	// Record the cost
	budget.CurrentDayCost = newDayCost
	budget.CurrentMonthCost = newMonthCost
	budget.TotalCost += cost

	return nil
}

// CheckBudget checks if a user can afford a certain cost without recording it
func (cbm *CostBudgetManager) CheckBudget(userID string, cost float64) error {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	budget, exists := cbm.budgets[userID]
	if !exists {
		// No budget set - allow by default
		return nil
	}

	now := time.Now()

	// Calculate what costs would be after this operation
	currentDayCost := budget.CurrentDayCost
	currentMonthCost := budget.CurrentMonthCost

	// Reset if needed (for checking purposes)
	if !isSameDay(budget.CurrentDay, now) {
		currentDayCost = 0
	}
	if !isSameMonth(budget.CurrentMonth, now) {
		currentMonthCost = 0
	}

	// Check daily budget
	if budget.DailyBudget > 0 && (currentDayCost + cost) > budget.DailyBudget {
		return fmt.Errorf("would exceed daily budget: %.4f/%.4f USD (this request: %.4f USD)",
			currentDayCost, budget.DailyBudget, cost)
	}

	// Check monthly budget
	if budget.MonthlyBudget > 0 && (currentMonthCost + cost) > budget.MonthlyBudget {
		return fmt.Errorf("would exceed monthly budget: %.4f/%.4f USD (this request: %.4f USD)",
			currentMonthCost, budget.MonthlyBudget, cost)
	}

	return nil
}

// ListBudgets returns all budgets (admin only)
func (cbm *CostBudgetManager) ListBudgets() []*CostBudget {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	budgets := make([]*CostBudget, 0, len(cbm.budgets))
	for _, budget := range cbm.budgets {
		budgetCopy := *budget
		budgets = append(budgets, &budgetCopy)
	}

	return budgets
}

// ResetDailyBudgets resets all daily budgets (should be called daily via cron)
func (cbm *CostBudgetManager) ResetDailyBudgets() {
	cbm.mu.Lock()
	defer cbm.mu.Unlock()

	now := time.Now()
	for _, budget := range cbm.budgets {
		if !isSameDay(budget.CurrentDay, now) {
			budget.CurrentDay = startOfDay(now)
			budget.CurrentDayCost = 0
			budget.LastResetDaily = now
		}
	}
}

// ResetMonthlyBudgets resets all monthly budgets (should be called monthly via cron)
func (cbm *CostBudgetManager) ResetMonthlyBudgets() {
	cbm.mu.Lock()
	defer cbm.mu.Unlock()

	now := time.Now()
	for _, budget := range cbm.budgets {
		if !isSameMonth(budget.CurrentMonth, now) {
			budget.CurrentMonth = startOfMonth(now)
			budget.CurrentMonthCost = 0
			budget.LastResetMonthly = now
		}
	}
}

// DeleteBudget removes a budget for a user
func (cbm *CostBudgetManager) DeleteBudget(userID string) error {
	cbm.mu.Lock()
	defer cbm.mu.Unlock()

	if _, exists := cbm.budgets[userID]; !exists {
		return fmt.Errorf("budget not found for user: %s", userID)
	}

	delete(cbm.budgets, userID)
	return nil
}

// Helper functions

// startOfDay returns the start of the day for a given time
func startOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

// startOfMonth returns the start of the month for a given time
func startOfMonth(t time.Time) time.Time {
	year, month, _ := t.Date()
	return time.Date(year, month, 1, 0, 0, 0, 0, t.Location())
}

// isSameDay checks if two times are on the same day
func isSameDay(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

// isSameMonth checks if two times are in the same month
func isSameMonth(t1, t2 time.Time) bool {
	y1, m1, _ := t1.Date()
	y2, m2, _ := t2.Date()
	return y1 == y2 && m1 == m2
}
