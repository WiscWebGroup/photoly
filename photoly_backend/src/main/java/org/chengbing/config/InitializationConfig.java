package org.chengbing.config;

import org.springframework.stereotype.Component;

import javax.annotation.PostConstruct;

***REMOVED***
 * @Author: Harold澂冰
 * @Date: 2022/7/15 15:44
***REMOVED***
@Component
public class InitializationConfig {

    @PostConstruct
    public void loadConfig()
***REMOVED***
        System.out.println("Here comes the config init");
***REMOVED***
***REMOVED***
