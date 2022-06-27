package org.chengbing.util;

import lombok.Data;

/**
 * @Author: Harold澂冰
 * @Date: 2022/6/24 16:17
 */
@Data
public class Result<T> {
    T t;
    Integer msgCode;

    public Result(T t, int i) {
        this.t = t;
        this.msgCode = i;
    }
}
