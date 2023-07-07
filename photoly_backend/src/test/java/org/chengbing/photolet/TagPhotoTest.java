package org.chengbing.photolet;

import org.chengbing.dao.TagPhotoMapper;
import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.SpringBootTest;

import javax.annotation.Resource;
import java.util.ArrayList;
import java.util.List;

/**
 * @Author: Harold澂冰
 * @Date: 2022/6/30 10:01
 */
@SpringBootTest
public class TagPhotoTest {

    @Resource
    TagPhotoMapper mapper;

    @Test
    public void test1()
    {
        List list = new ArrayList<>();
        list.add(1);
        list.add(2);
        list.add(3);
        System.out.println(mapper.selectByTags(3, list, 3));
    }
}
