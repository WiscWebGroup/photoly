package org.chengbing.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import java.time.LocalDateTime;
import java.io.Serializable;
import io.swagger.annotations.ApiModel;
import io.swagger.annotations.ApiModelProperty;
import lombok.Data;
import lombok.EqualsAndHashCode;

***REMOVED***
 * <p>
 * 
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
***REMOVED***
@Data
@EqualsAndHashCode(callSuper = false)
@ApiModel(value="User对象", description="")
public class User implements Serializable {

    private static final long serialVersionUID = 1L;

    @TableId(value = "user_id", type = IdType.AUTO)
    private Integer userId;

    private String userName;

    private String email;

    private LocalDateTime createDate;

    private String role;

    private String password;

    private String uuid;

    public User(String email, String password) {
        this.email = email;
        this.password = password;
***REMOVED***
***REMOVED***
