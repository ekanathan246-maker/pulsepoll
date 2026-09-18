import { expect, test } from '@playwright/test'

test('host creates a poll and a guest vote updates both viewers live', async ({ browser }, testInfo) => {
	const host = await browser.newContext()
	const hostPage = await host.newPage()
	const email = `host-${testInfo.project.name}-${Date.now()}-${Math.random().toString(36).slice(2)}@example.test`

	await hostPage.goto('/signup')
	await hostPage.getByLabel('Name').fill('Live Demo Host')
	await hostPage.getByLabel('Email').fill(email)
	await hostPage.getByLabel('Password').fill('Interview!2026')
	await hostPage.getByRole('button', { name: 'Sign up' }).click()
	await expect(hostPage).toHaveURL(/\/dashboard$/)

	await hostPage.getByRole('link', { name: /New poll/ }).click()
	await hostPage.getByLabel('Question').fill('Which engineering proof wins the interview?')
	await hostPage.getByPlaceholder('Option 1').fill('Durable votes')
	await hostPage.getByPlaceholder('Option 2').fill('Pretty gradients')
	await hostPage.getByRole('button', { name: 'Create poll' }).click()
	await expect(hostPage.getByRole('heading', { name: 'Which engineering proof wins the interview?' })).toBeVisible()
	const pollURL = hostPage.url()

	const guest = await browser.newContext()
	const guestPage = await guest.newPage()
	await guestPage.goto(pollURL)
	await expect(guestPage.getByText('0 (0%)')).toHaveCount(0)
	await guestPage.getByRole('button', { name: 'Durable votes' }).click()
	await expect(guestPage.getByText(/Your vote/)).toBeVisible()
	await expect(guestPage.getByText('1 (100%)')).toBeVisible()

	await expect(hostPage.getByText('1 (100%)')).toBeVisible({ timeout: 10_000 })
	await expect(hostPage.getByText(/Live · synced/i)).toBeVisible()

	await hostPage.goto('/dashboard')
	const pollCard = hostPage.locator('.card').filter({ hasText: 'Which engineering proof wins the interview?' })
	await expect(pollCard.getByRole('link', { name: 'Export CSV' })).toHaveAttribute('href', /export\.csv$/)
	await pollCard.getByRole('button', { name: 'Close' }).click()
	await expect(guestPage.getByText('Closed', { exact: true })).toBeVisible({ timeout: 10_000 })

	const lateGuest = await browser.newContext()
	const latePage = await lateGuest.newPage()
	await latePage.goto(pollURL)
	await expect(latePage.getByRole('button', { name: /Durable votes/ })).toBeDisabled()

	await lateGuest.close()
	await guest.close()
	await host.close()
})
