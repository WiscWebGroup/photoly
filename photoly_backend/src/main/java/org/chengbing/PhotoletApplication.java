package org.chengbing;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.context.annotation.ComponentScan;
import org.springframework.web.servlet.config.annotation.EnableWebMvc;
import springfox.documentation.swagger2.annotations.EnableSwagger2;

@SpringBootApplication
@ComponentScan("org.chengbing.dao")
@ComponentScan("org.chengbing.controller")
@EnableSwagger2
public class PhotoletApplication {

    public static void main(String[] args) {
        SpringApplication.run(PhotoletApplication.class, args);
    }

}
