from selenium import webdriver

driver = webdriver.Chrome()
driver.get('http://www.google.com/');
time.sleep(5)
search_box = driver.find_element_by_name('q')
search_box.send_keys('ChromeDriver')
search_box.submit()
driver.quit()
