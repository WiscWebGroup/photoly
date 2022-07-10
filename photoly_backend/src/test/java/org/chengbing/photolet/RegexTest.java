package org.chengbing.photolet;

import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * @Author: Harold澂冰
 * @Date: 2022/7/10 17:18
 */
public class RegexTest {

    public static void main(String[] args) {
        Pattern pattern = Pattern.compile(".+@.+\\..+");
        Matcher matcher = pattern.matcher("2@1.wd");
        System.out.println(matcher.find());
    }
}
