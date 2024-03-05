![example workflow](https://github.com/WiscWebGroup/photoly/actions/workflows/codeql-analysis.yml/badge.svg)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FWiscWebGroup%2Fphotoly.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2FWiscWebGroup%2Fphotoly?ref=badge_shield)
# Photoly
<img src="https://raw.githubusercontent.com/Harold-y/photoly/main/Logo.png" height="160px" width="160px" />
<br>

This is a picture host application. There are demands for the self-host and reliable picture-host applications. Photoly provides a solution just right for that. Using Photoly, you can manage your pictures and photos with easy clicks, and categorize them using tags, galleries, and namespaces. We are also sure that the search functionality could make it easier to see the photos you are interested in. Moreover, Photoly provides API for you to access our functionalities for embedding in other applications.

Photoly是一款非常美丽的图床网页应用。随着人们对可靠的 自助图片托管应用需求增加，Photoly应运而生。使用 Photoly，您可以轻松管理图片和视频，并使用标签、图库和文件夹对图片和视频进行分类；利用搜索功能，充分探索您的媒体库。此外，Photoly 还提供 API，方便您将我们的功能嵌入到其他应用程序中。

## Functionalities
- Upload Photo / Video, and store them in folders
- Attach a Photo / Video with a Gallery or a Tag
- Share a Photo / Video using a share link, or a QR code
- Automatic record photo's Exif information, including GPS location, Camera Model, Lens Model, ISO, Shutter, Aperture, etc.
- Display a Photo Location Map
- Search Photos based on various conditions
- Create API to operate your photo using REST API

## Screenshots
<img src="https://github.com/WiscWebGroup/photoly/assets/68500948/3304e591-7b8a-49d2-a483-61e04206e8dd" style="width: 800px;" />

<img src="https://github.com/WiscWebGroup/photoly/assets/68500948/bd6cc12e-dc89-4e98-9cf7-2eaf35ba95b7" style="width: 800px;" />

<img src="https://github.com/WiscWebGroup/photoly/assets/68500948/1518dfcc-eee2-4c5e-b686-aa1c5cdb57ae" style="width: 800px;" />

<img src="https://github.com/WiscWebGroup/photoly/assets/68500948/4219d76f-5e9b-4587-ae4b-11379107fb35" style="width: 800px;" />

<img src="https://github.com/WiscWebGroup/photoly/assets/68500948/44d94933-d7e7-4617-9fa5-4b2cc972da9c" style="width: 800px;" />

<img src="https://github.com/WiscWebGroup/photoly/assets/68500948/23ae3e12-e465-4c3b-b642-c9b2de01c5a5" style="width: 800px;" />

<img src="https://github.com/WiscWebGroup/photoly/assets/68500948/72e5adf6-d4bb-4f20-bd20-d6c7ea7921c8" style="width: 800px;" />

<img src="https://github.com/WiscWebGroup/photoly/assets/68500948/7acbc34e-30c0-4115-8607-9cfc8c9396ca" style="width: 800px;" />

## Specifications

DB: MySQL + Redis

Backend: Spring Boot

Frontend: Vue3 (new version of frontend) + Naive UI

## How to Run?

### Docker Version

1. Make sure that you have docker and docker-compose installed in your VM / Host
2. Create a folder named "mysql-data" in the photoly/database folder (make sure photoly/database/mysql-data exist and is empty first time you start it)
3. cd photoly and execute this command
```
root@cbr-gov-frostland1-dev:/test/photoly$ > docker-compose up
```
4. Access localhost:5200

*Note: the initial MySQL will take longer to execute the initialization ("photoly\database\mysql-dump"), and the data will persist to ("photoly\database\mysql-data"). Redis data (login tokens) do not persist because we don't have to. In the docker-compose.yml file and two Dockerfiles inside photoly_backend and photoly_frontend_vue, we have defined (hardcoded) the ports, names, etc. so if you just want to conveniently run the photoly, try not to change these settings. 



However, if you decide to change some settings, make sure to go over photoly_backend\src\main\resources\application.properties, photoly_frontend_vue\.env, photoly_backend\Dockerfile, photoly_frontend_vue\Dockerfile, photoly_frontend_vue\nginx_conf, and .\docker-compose.yml.

### Normal VM/Host Version

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
