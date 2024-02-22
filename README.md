![example workflow](https://github.com/WiscWebGroup/photoly/actions/workflows/codeql-analysis.yml/badge.svg)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FWiscWebGroup%2Fphotoly.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2FWiscWebGroup%2Fphotoly?ref=badge_shield)
# Photoly
<img src="https://raw.githubusercontent.com/Harold-y/photoly/main/Logo.png" height="160px" width="160px" />
<br>

This is a picture host application. There are demands for the self-host and reliable picture-host applications. Photoly provides a solution just right for that. Using Photoly, you can manage your pictures and photos with easy clicks, and categorize them using tags, galleries, and namespaces. Moreover, Photoly provides API for you to access our functionalities for embedding in other applications.

Photoly是一款非常美丽的图床网页应用。随着人们对自助托管和可靠的图片托管应用程序需求增加，Photoly应运而生。使用 Photoly，您可以轻松点击管理图片和照片，并使用标签、图库和命名空间对图片和照片进行分类。此外，Photoly 还提供 API，方便您将我们的功能嵌入到其他应用程序中。

## Screenshots
<img src="https://github.com/WiscWebGroup/photoly/assets/68500948/3304e591-7b8a-49d2-a483-61e04206e8dd" style="width: 800px;" />

<img src="https://github.com/WiscWebGroup/photoly/assets/68500948/98744546-4840-4f23-94a1-229082def337" style="width: 800px;" />

<img src="https://github.com/WiscWebGroup/photoly/assets/68500948/9abba9c7-c475-4463-91de-1da18b49a5ae" style="width: 800px;" />

<img src="https://github.com/WiscWebGroup/photoly/assets/68500948/72e5adf6-d4bb-4f20-bd20-d6c7ea7921c8" style="width: 800px;" />

<img src="https://github.com/WiscWebGroup/photoly/assets/68500948/7acbc34e-30c0-4115-8607-9cfc8c9396ca" style="width: 800px;" />

## Specifications

DB: MySQL + Redis

Backend: Spring Boot

Frontend: Vue3 (new version of frontend) + Naive UI

## How to Run?

1. Edit application.properties in the backend to load your MySQL and Redis addresses, as well as your upload addresses
2. Edit .env in the frontend folder to fit in your backend address
3. In the backend folder, do
   ```
   mvn clean package -Dmaven.test.skip=true
   java -jar <path>/photoly.war
   ```
4. In the frontend folder, do
   ```
   npm install
   npm run build
   ```
   and then, use nginx to serve the frontend_vue/dist folder
5. Please refer to the Jenkinsfile for more examples





## License
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FWiscWebGroup%2Fphotoly.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2FWiscWebGroup%2Fphotoly?ref=badge_large)
