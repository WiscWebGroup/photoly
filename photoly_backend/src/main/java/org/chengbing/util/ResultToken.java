package org.chengbing.util;

import lombok.Data;

/**
 * @Author: Harold澂冰
 * @Date: 2022/6/26 21:21
 */
@Data
public class ResultToken<T> {
    T t;
    String token;
    Integer msgCode;

    public ResultToken(T t, String token, Integer msgCode) {
        this.t = t;
        this.token = token;
        this.msgCode = msgCode;
    }

    public ResultToken(String token, Integer msgCode) {
        this.token = token;
        this.msgCode = msgCode;
    }
}
