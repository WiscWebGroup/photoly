package org.chengbing.service.impl;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import org.chengbing.entity.User;
import org.chengbing.dao.UserMapper;
import org.chengbing.service.IUserService;
import com.baomidou.mybatisplus.extension.service.impl.ServiceImpl;
import org.chengbing.util.AESUtil;
import org.springframework.stereotype.Service;

import javax.annotation.Resource;
import java.util.List;

/**
 * <p>
 *  服务实现类
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
 */
@Service
public class UserServiceImpl extends ServiceImpl<UserMapper, User> implements IUserService {

    @Resource
    UserMapper mapper;

    @Override
    public List<User> selectUsers(String email) {
        QueryWrapper<User> wrapper = new QueryWrapper<>();
        wrapper.eq("email", email);
        return mapper.selectList(wrapper);
    }

    @Override
    public User selectUserByEmail(String email) {
        QueryWrapper<User> wrapper = new QueryWrapper<>();
        wrapper.eq("email", email);
        return mapper.selectOne(wrapper);
    }

    @Override
    public User selectUserToLogin(User user) {
        String email = user.getEmail();
        String password = AESUtil.aesEncryptStr(user.getPassword(), AESUtil.getPkey());
        QueryWrapper<User> wrapper = new QueryWrapper<>();
        wrapper.eq("password", password);
        wrapper.eq("email", email);
        return mapper.selectOne(wrapper);
    }
}
