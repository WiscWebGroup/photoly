package org.chengbing.util;

import org.springframework.context.annotation.Bean;
import org.springframework.data.redis.core.RedisTemplate;
import org.springframework.stereotype.Component;

import javax.annotation.Resource;
import javax.servlet.http.HttpServletRequest;

/**
 * @Author: Harold澂冰
 * @Date: 2022/6/29 10:14
 */
@Component
public class UserIdentity {
    @Resource
    RedisTemplate<String, Object> template;

    public Integer verifyUser(HttpServletRequest request)
    {
        String key = request.getHeader("HRD-Token");
        if(key == null || key.equals(""))
            return -1;
        Integer id = (Integer) template.opsForValue().get(key);
        if (id == null)
            return -2;
        return id;
    }
}
