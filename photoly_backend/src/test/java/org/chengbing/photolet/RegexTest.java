package org.chengbing.photolet;

import java.util.regex.Matcher;
import java.util.regex.Pattern;

***REMOVED***
 * @Author: Harold澂冰
 * @Date: 2022/7/10 17:18
***REMOVED***
public class RegexTest {

    public static void main(String[] args) {
       /* Pattern pattern = Pattern.compile(".+@.+\\..+");
        Matcher matcher = pattern.matcher("2@1.wd");
        System.out.println(matcher.find());*/

        Pattern pattern = Pattern.compile("//(.*?)/");
        Matcher matcher = pattern.matcher("jdbc:mysql://localhost:3306/picbed?serverTimezone=Asia/Shanghai");
        String mysqlAddress = "";
        while (matcher.find()) {
            mysqlAddress = matcher.group(1);
***REMOVED***
        System.out.println(mysqlAddress);
***REMOVED***
***REMOVED***
