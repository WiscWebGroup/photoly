package org.chengbing.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
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
@ApiModel(value="Cred对象", description="")
public class Cred implements Serializable {

    private static final long serialVersionUID = 1L;

    @TableId(value = "cred_id", type = IdType.AUTO)
    private Integer credId;

    private Integer userId;

    private String token;

    private String authorization;


***REMOVED***
