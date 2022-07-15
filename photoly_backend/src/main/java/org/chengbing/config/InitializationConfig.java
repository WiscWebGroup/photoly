package org.chengbing.config;

import org.springframework.stereotype.Component;

import javax.annotation.PostConstruct;

/**
 * @Author: Harold澂冰
 * @Date: 2022/7/15 15:44
 */
@Component
public class InitializationConfig {

    @PostConstruct
    public void loadConfig()
    {
        System.out.println("Here comes the config init");
    }
}
