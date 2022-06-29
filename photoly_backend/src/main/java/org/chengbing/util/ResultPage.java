***REMOVED***

import lombok.Data;

***REMOVED***
 * @Author: Harold澂冰
 * @Date: 2022/6/29 10:25
***REMOVED***
@Data
public class ResultPage<T> {
    T t;
    Integer pageNum;
    Integer msgCode;

    public ResultPage(T t, Integer pageNum, Integer msgCode) {
        this.t = t;
        this.pageNum = pageNum;
        this.msgCode = msgCode;
***REMOVED***

    public ResultPage(Integer pageNum, Integer msgCode) {
        this.pageNum = pageNum;
        this.msgCode = msgCode;
***REMOVED***
***REMOVED***
