package org.chengbing.util;

import lombok.Data;

/**
 * @Author: Harold澂冰
 * @Date: 2022/6/29 10:25
 */
@Data
public class ResultPage<T> {
    T t;
    Integer pageNum;
    Integer msgCode;

    public ResultPage(T t, Integer pageNum, Integer msgCode) {
        this.t = t;
        this.pageNum = pageNum;
        this.msgCode = msgCode;
    }

    public ResultPage(Integer pageNum, Integer msgCode) {
        this.pageNum = pageNum;
        this.msgCode = msgCode;
    }
}
