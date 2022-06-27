package org.chengbing.config;
import com.baomidou.mybatisplus.core.incrementer.IKeyGenerator;
import com.baomidou.mybatisplus.extension.incrementer.H2KeyGenerator;
import com.baomidou.mybatisplus.extension.plugins.PaginationInterceptor;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
***REMOVED***
 * @Author: Harold澂冰
 * @Date: 2022/6/24 15:10
***REMOVED***
@Configuration
public class MybatisPlusConfig {
    ***REMOVED***
***REMOVED*** 注入主键生成器***REMOVED*** @Bean
    public IKeyGenerator keyGenerator() {
        return new H2KeyGenerator();
***REMOVED***
    ***REMOVED***
***REMOVED*** 分页插件***REMOVED*** @Bean
    public PaginationInterceptor paginationInterceptor() {
        return new PaginationInterceptor();
***REMOVED***
***REMOVED***
