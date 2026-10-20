"""Waits for the IceHive SPA to connect and render data before capture."""
import time

from selenium.webdriver.common.by import By


def run(driver):
    deadline = time.time() + 15
    while time.time() < deadline:
        rows = driver.find_elements(By.CSS_SELECTOR, "main tbody tr, main li, main dl, main form")
        if rows:
            break
        time.sleep(0.25)
    driver.execute_script("window.scrollTo(0, 0);")
