package org.chengbing.config;
import com.baomidou.mybatisplus.core.incrementer.IKeyGenerator;
import com.baomidou.mybatisplus.extension.incrementer.H2KeyGenerator;
import com.baomidou.mybatisplus.extension.plugins.PaginationInterceptor;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
/**
 * @Author: Harold澂冰
 * @Date: 2022/6/24 15:10
 */
@Configuration
public class MybatisPlusConfig {
    /**
     * 注入主键生成器 */ @Bean
    public IKeyGenerator keyGenerator() {
        return new H2KeyGenerator();
    }
    /**
     * 分页插件 */ @Bean
    public PaginationInterceptor paginationInterceptor() {
        return new PaginationInterceptor();
    }
}
