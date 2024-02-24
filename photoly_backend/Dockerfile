FROM adoptopenjdk/maven-openjdk11:latest

COPY . /app/
RUN mkdir -p /photoly

WORKDIR /app/
RUN mvn clean package -Dmaven.test.skip=true
RUN chmod 777 target/photoly.war

CMD ["nohup",  "java",  "-jar",  "target/photoly.war",  ">",  "/photoly/photoly.log",  "2>1&"]