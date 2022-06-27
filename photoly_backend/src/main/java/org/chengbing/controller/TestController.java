package org.chengbing.controller;

import io.swagger.annotations.Api;
import io.swagger.annotations.ApiModel;
import io.swagger.annotations.ApiOperation;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.ArrayList;
import java.util.List;

/**
 * @Author: Harold澂冰
 * @Date: 2022/6/24 16:04
 */
@Api
@RestController
@RequestMapping("/test")
public class TestController {

    @RequestMapping("/areturn")
    @ApiOperation("ac")
    public List<Object> returnAll()
    {
        return new ArrayList<>();
    }
}
